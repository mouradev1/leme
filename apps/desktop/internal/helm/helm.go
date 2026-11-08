// Package helm uses the Helm v3 Go SDK in-process (no helm binary required),
// reusing the cluster Session's rest.Config, discovery cache and REST mapper.
package helm

import (
	"context"
	"fmt"
	"leme/internal/platform/i18n"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"leme/internal/cluster"
)

type Release struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Revision     int    `json:"revision"`
	Status       string `json:"status"`
	Chart        string `json:"chart"`
	ChartVersion string `json:"chartVersion"`
	AppVersion   string `json:"appVersion"`
	Description  string `json:"description"`
	Updated      string `json:"updated"` // RFC3339 (Wails v2 TS generator cannot map time.Time)
}

type InstallRequest struct {
	ReleaseName     string `json:"releaseName"`
	Namespace       string `json:"namespace"`
	Chart           string `json:"chart"`   // "nginx", "./path", or "oci://registry/chart"
	RepoURL         string `json:"repoURL"` // e.g. https://charts.bitnami.com/bitnami
	Version         string `json:"version"`
	ValuesYAML      string `json:"valuesYAML"`
	CreateNamespace bool   `json:"createNamespace"`
	Wait            bool   `json:"wait"`
}

// UpgradeRequest changes a release: new values and/or a new chart version.
// Chart empty = keep the release's current chart (values-only upgrade).
type UpgradeRequest struct {
	ReleaseName string `json:"releaseName"`
	Namespace   string `json:"namespace"`
	Chart       string `json:"chart"`
	RepoURL     string `json:"repoURL"`
	Version     string `json:"version"`
	ValuesYAML  string `json:"valuesYAML"` // the release's user values, replaced as a whole
	Wait        bool   `json:"wait"`
}

// checkRepo: charts from the network only over https or OCI registries.
func checkRepo(repoURL, chart string) error {
	if repoURL != "" && !strings.HasPrefix(repoURL, "https://") {
		return fmt.Errorf(i18n.T("use um repositório https:// (recebido %q)", "use an https:// repository (got %q)"), repoURL)
	}
	if strings.Contains(chart, "://") && !strings.HasPrefix(chart, "oci://") && !strings.HasPrefix(chart, "https://") {
		return fmt.Errorf(i18n.T("chart remoto só por https:// ou oci:// (recebido %q)", "remote charts only over https:// or oci:// (got %q)"), chart)
	}
	return nil
}

func parseValues(s string) (map[string]any, error) {
	vals := map[string]any{}
	if err := yaml.Unmarshal([]byte(s), &vals); err != nil {
		return nil, fmt.Errorf("values: %w", err)
	}
	if vals == nil {
		vals = map[string]any{}
	}
	return vals, nil
}

type Client struct{ s *cluster.Session }

func New(s *cluster.Session) *Client { return &Client{s: s} }

// config builds an action.Configuration scoped to ns ("" = all namespaces).
// Releases are stored by Helm in Secrets (the default driver).
func (c *Client) config(ns string) (*action.Configuration, error) {
	cfg := new(action.Configuration)
	if err := cfg.Init(getter{c.s}, ns, "secret", func(string, ...any) {}); err != nil {
		return nil, err
	}
	rc, err := registry.NewClient()
	if err != nil {
		return nil, err
	}
	cfg.RegistryClient = rc
	return cfg, nil
}

func (c *Client) List(ns string) ([]Release, error) {
	cfg, err := c.config(ns)
	if err != nil {
		return nil, err
	}
	l := action.NewList(cfg)
	l.AllNamespaces = ns == ""
	l.All = true // include failed/pending releases too
	rels, err := l.Run()
	if err != nil {
		return nil, err
	}
	return toDTOs(rels), nil
}

func (c *Client) History(ns, name string) ([]Release, error) {
	cfg, err := c.config(ns)
	if err != nil {
		return nil, err
	}
	rels, err := action.NewHistory(cfg).Run(name)
	if err != nil {
		return nil, err
	}
	return toDTOs(rels), nil
}

func (c *Client) Values(ns, name string, all bool) (string, error) {
	cfg, err := c.config(ns)
	if err != nil {
		return "", err
	}
	gv := action.NewGetValues(cfg)
	gv.AllValues = all
	vals, err := gv.Run(name)
	if err != nil {
		return "", err
	}
	b, err := yaml.Marshal(vals)
	return string(b), err
}

func (c *Client) Rollback(ns, name string, revision int) error {
	cfg, err := c.config(ns)
	if err != nil {
		return err
	}
	rb := action.NewRollback(cfg)
	rb.Version = revision // 0 = previous revision
	return rb.Run(name)
}

func (c *Client) Uninstall(ns, name string) error {
	cfg, err := c.config(ns)
	if err != nil {
		return err
	}
	_, err = action.NewUninstall(cfg).Run(name)
	return err
}

func (c *Client) Install(ctx context.Context, r InstallRequest) (Release, error) {
	if err := checkRepo(r.RepoURL, r.Chart); err != nil {
		return Release{}, err
	}
	cfg, err := c.config(r.Namespace)
	if err != nil {
		return Release{}, err
	}
	in := action.NewInstall(cfg)
	in.ReleaseName, in.Namespace, in.CreateNamespace = r.ReleaseName, r.Namespace, r.CreateNamespace
	in.RepoURL, in.Version = r.RepoURL, r.Version
	in.Wait, in.Timeout = r.Wait, 5*time.Minute
	in.SetRegistryClient(cfg.RegistryClient)

	path, err := in.ChartPathOptions.LocateChart(r.Chart, cli.New()) // downloads into helm's cache
	if err != nil {
		return Release{}, fmt.Errorf("locate chart: %w", err)
	}
	ch, err := loader.Load(path)
	if err != nil {
		return Release{}, err
	}
	vals, err := parseValues(r.ValuesYAML)
	if err != nil {
		return Release{}, err
	}
	rel, err := in.RunWithContext(ctx, ch, vals)
	if err != nil {
		return Release{}, err
	}
	return toDTO(rel), nil
}

// Upgrade applies new values (and optionally another chart version).
func (c *Client) Upgrade(ctx context.Context, r UpgradeRequest) (Release, error) {
	if err := checkRepo(r.RepoURL, r.Chart); err != nil {
		return Release{}, err
	}
	cfg, err := c.config(r.Namespace)
	if err != nil {
		return Release{}, err
	}
	vals, err := parseValues(r.ValuesYAML)
	if err != nil {
		return Release{}, err
	}
	up := action.NewUpgrade(cfg)
	up.Namespace, up.RepoURL, up.Version = r.Namespace, r.RepoURL, r.Version
	up.Wait, up.Timeout = r.Wait, 5*time.Minute
	up.SetRegistryClient(cfg.RegistryClient)

	var ch *chart.Chart
	if r.Chart == "" {
		cur, err := action.NewGet(cfg).Run(r.ReleaseName)
		if err != nil {
			return Release{}, err
		}
		ch = cur.Chart
	} else {
		path, err := up.ChartPathOptions.LocateChart(r.Chart, cli.New())
		if err != nil {
			return Release{}, fmt.Errorf("locate chart: %w", err)
		}
		if ch, err = loader.Load(path); err != nil {
			return Release{}, err
		}
	}
	rel, err := up.RunWithContext(ctx, r.ReleaseName, ch, vals)
	if err != nil {
		return Release{}, err
	}
	return toDTO(rel), nil
}

func toDTOs(rels []*release.Release) []Release {
	out := make([]Release, 0, len(rels))
	for _, r := range rels {
		out = append(out, toDTO(r))
	}
	return out
}

func toDTO(r *release.Release) Release {
	d := Release{Name: r.Name, Namespace: r.Namespace, Revision: r.Version}
	if r.Info != nil {
		d.Status, d.Description, d.Updated = r.Info.Status.String(), r.Info.Description, r.Info.LastDeployed.Format(time.RFC3339)
	}
	if r.Chart != nil && r.Chart.Metadata != nil {
		d.Chart, d.ChartVersion, d.AppVersion = r.Chart.Metadata.Name, r.Chart.Metadata.Version, r.Chart.Metadata.AppVersion
	}
	return d
}

// getter adapts a Session to genericclioptions.RESTClientGetter so Helm shares
// our already-warm discovery cache instead of building its own.
type getter struct{ s *cluster.Session }

func (g getter) ToRESTConfig() (*rest.Config, error) { return rest.CopyConfig(g.s.Config), nil }
func (g getter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return g.s.Discovery, nil
}
func (g getter) ToRESTMapper() (meta.RESTMapper, error)        { return g.s.Mapper, nil }
func (g getter) ToRawKubeConfigLoader() clientcmd.ClientConfig { return g.s.ClientConfig }
