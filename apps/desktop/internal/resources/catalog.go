// Package resources is generic over every Kubernetes kind, built-in or CRD.
// Nothing here has per-kind Go types: the API server's discovery, Table
// printing and server-side validation do the kind-specific work.
package resources

import (
	"slices"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	"leme/internal/cluster"
)

// Ref identifies a resource collection. It crosses the Wails bridge as JSON.
type Ref struct {
	Group      string `json:"group"`
	Version    string `json:"version"`
	Resource   string `json:"resource"`
	Namespaced bool   `json:"namespaced"`
}

func (r Ref) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: r.Group, Version: r.Version, Resource: r.Resource}
}

// Path builds the REST collection path; ns == "" means all namespaces.
func (r Ref) Path(ns string) string {
	p := "/api/" + r.Version
	if r.Group != "" {
		p = "/apis/" + r.Group + "/" + r.Version
	}
	if r.Namespaced && ns != "" {
		p += "/namespaces/" + ns
	}
	return p + "/" + r.Resource
}

type Kind struct {
	Ref
	Kind       string   `json:"kind"`
	Category   string   `json:"category"`
	Verbs      []string `json:"verbs"`
	ShortNames []string `json:"shortNames"`
}

// Sidebar sections. Anything unknown in a non-k8s.io group is a CRD.
var categories = map[string]string{
	"/nodes": "Nodes", "/namespaces": "Namespaces", "/events": "Events",
	"/pods": "Workloads", "apps/deployments": "Workloads", "apps/daemonsets": "Workloads",
	"apps/statefulsets": "Workloads", "apps/replicasets": "Workloads",
	"batch/jobs": "Workloads", "batch/cronjobs": "Workloads",
	"/configmaps": "Config", "/secrets": "Config", "/resourcequotas": "Config", "/limitranges": "Config",
	"autoscaling/horizontalpodautoscalers": "Config", "policy/poddisruptionbudgets": "Config",
	"scheduling.k8s.io/priorityclasses": "Config", "node.k8s.io/runtimeclasses": "Config",
	"/services": "Network", "/endpoints": "Network", "networking.k8s.io/ingresses": "Network",
	"networking.k8s.io/ingressclasses": "Network", "networking.k8s.io/networkpolicies": "Network",
	"discovery.k8s.io/endpointslices": "Network",
	"/persistentvolumes":              "Storage", "/persistentvolumeclaims": "Storage",
	"storage.k8s.io/storageclasses": "Storage",
	"/serviceaccounts":              "Access Control", "rbac.authorization.k8s.io/roles": "Access Control",
	"rbac.authorization.k8s.io/clusterroles":         "Access Control",
	"rbac.authorization.k8s.io/rolebindings":         "Access Control",
	"rbac.authorization.k8s.io/clusterrolebindings":  "Access Control",
	"apiextensions.k8s.io/customresourcedefinitions": "Custom Resources",
}

func category(group, resource string) string {
	if c, ok := categories[group+"/"+resource]; ok {
		return c
	}
	if group == "" || strings.HasSuffix(group, ".k8s.io") || !strings.Contains(group, ".") {
		return "Other"
	}
	return "Custom Resources"
}

// Catalog lists every listable kind the cluster serves (preferred version
// only). Partial discovery failures (a broken aggregated API such as a dead
// metrics-server) must not hide the rest of the cluster, so they're tolerated.
func Catalog(s *cluster.Session) ([]Kind, error) {
	lists, err := s.Discovery.ServerPreferredResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, err
	}
	var out []Kind
	for _, l := range lists {
		gv, err := schema.ParseGroupVersion(l.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range l.APIResources {
			if strings.Contains(r.Name, "/") || !slices.Contains(r.Verbs, "list") {
				continue // subresources (pods/log, deployments/scale) and non-listables
			}
			out = append(out, Kind{
				Ref:  Ref{Group: gv.Group, Version: gv.Version, Resource: r.Name, Namespaced: r.Namespaced},
				Kind: r.Kind, Category: category(gv.Group, r.Name),
				Verbs: r.Verbs, ShortNames: r.ShortNames,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

// InvalidateCatalog forces rediscovery, e.g. after a CRD is installed.
func InvalidateCatalog(s *cluster.Session) {
	s.Discovery.Invalidate()
	s.Mapper.Reset()
}
