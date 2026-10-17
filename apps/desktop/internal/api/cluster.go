package api

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"leme/internal/cluster"
	"leme/internal/platform/settings"
)

type ClusterAPI struct{ d *Deps }

type ServerInfo struct {
	Context          string `json:"context"`
	Version          string `json:"version"`
	Server           string `json:"server"`
	DefaultNamespace string `json:"defaultNamespace"`
}

// AppVersion is the build version (e.g. "0.3.1", or "dev" for local builds).
func (a *ClusterAPI) AppVersion() string { return a.d.Version }

func (a *ClusterAPI) ListContexts() []cluster.ContextInfo { return a.d.Clusters.Contexts() }

func (a *ClusterAPI) Connect(name string) (ServerInfo, error) {
	s, err := a.d.session(name)
	if err != nil {
		return ServerInfo{}, err
	}
	return ServerInfo{Context: name, Version: s.Version, Server: s.Config.Host, DefaultNamespace: s.DefaultNamespace()}, nil
}

func (a *ClusterAPI) Disconnect(name string) { a.d.Clusters.Disconnect(name) }

// KubeconfigFiles lists every file feeding the context list.
func (a *ClusterAPI) KubeconfigFiles() []cluster.KubeconfigFile { return a.d.Clusters.Files() }

// AddKubeconfig adds a kubeconfig from any path (typed/pasted in the UI).
// Returns how many contexts the file defines. Persisted across restarts.
func (a *ClusterAPI) AddKubeconfig(path string) (int, error) {
	n, err := a.d.Clusters.AddPath(path)
	if err != nil {
		return 0, err
	}
	return n, a.persist()
}

// PickKubeconfig opens the native file dialog (hidden files visible, since
// kubeconfigs usually live in dot-folders) and adds every selected file.
// Returns the total number of contexts added; 0 if the user cancelled.
func (a *ClusterAPI) PickKubeconfig() (int, error) {
	home, _ := os.UserHomeDir()
	files, err := runtime.OpenMultipleFilesDialog(a.d.Bus.Context(), runtime.OpenDialogOptions{
		Title:            "Selecione arquivo(s) kubeconfig",
		DefaultDirectory: filepath.Join(home, ".kube"),
		ShowHiddenFiles:  true,
	})
	if err != nil {
		return 0, err
	}
	total := 0
	var errs []error
	for _, f := range files {
		n, err := a.d.Clusters.AddPath(f)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		total += n
	}
	if err := a.persist(); err != nil {
		errs = append(errs, err)
	}
	return total, errors.Join(errs...)
}

// SaveKubeconfig stores pasted kubeconfig YAML in the app's private dir and
// adds its clusters (persisted across restarts like any added file).
func (a *ClusterAPI) SaveKubeconfig(name, content string) (cluster.SavedKubeconfig, error) {
	saved, err := a.d.Clusters.SaveKubeconfig(name, content)
	if err != nil {
		return saved, err
	}
	return saved, a.persist()
}

func (a *ClusterAPI) RemoveKubeconfig(path string) error {
	if err := a.d.Clusters.RemovePath(path); err != nil {
		return err
	}
	return a.persist()
}

func (a *ClusterAPI) persist() error {
	return settings.Save(settings.Settings{KubeconfigPaths: a.d.Clusters.ExtraPaths()})
}

// Namespaces falls back to the context's namespace when the user lacks
// cluster-wide list permission (common in multi-tenant clusters).
func (a *ClusterAPI) Namespaces(name string) ([]string, error) {
	s, err := a.d.session(name)
	if err != nil {
		return nil, err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	l, err := s.Kube.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return []string{s.DefaultNamespace()}, nil
	}
	out := make([]string, 0, len(l.Items))
	for _, n := range l.Items {
		out = append(out, n.Name)
	}
	sort.Strings(out)
	return out, nil
}
