package resources

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/metadata"

	"leme/internal/cluster"
)

// SearchHit is one object found by name (for the command palette).
type SearchHit struct {
	Kind      string `json:"kind"` // e.g. "Deployment"
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type searchKind struct {
	Kind string
	Ref  Ref
}

// The kinds people jump to by name, most useful first.
var searchKinds = []searchKind{
	{"Pod", Ref{"", "v1", "pods", true}},
	{"Deployment", Ref{"apps", "v1", "deployments", true}},
	{"StatefulSet", Ref{"apps", "v1", "statefulsets", true}},
	{"DaemonSet", Ref{"apps", "v1", "daemonsets", true}},
	{"Service", Ref{"", "v1", "services", true}},
	{"Ingress", Ref{"networking.k8s.io", "v1", "ingresses", true}},
	{"CronJob", Ref{"batch", "v1", "cronjobs", true}},
	{"Job", Ref{"batch", "v1", "jobs", true}},
	{"ConfigMap", Ref{"", "v1", "configmaps", true}},
	{"Secret", Ref{"", "v1", "secrets", true}},
	{"PersistentVolumeClaim", Ref{"", "v1", "persistentvolumeclaims", true}},
	{"Node", Ref{"", "v1", "nodes", false}},
	{"Namespace", Ref{"", "v1", "namespaces", false}},
}

// Names are listed through the metadata API (name/namespace only, never the
// object body, so Secret values are not downloaded) and kept for a few
// seconds, so typing in the palette doesn't list again on every key.
type nameList struct {
	at    time.Time
	items [][2]string // namespace, name
}

var (
	searchMu    sync.Mutex
	searchCache = map[string]nameList{}
)

const searchTTL = 15 * time.Second

func listNames(ctx context.Context, mc metadata.Interface, s *cluster.Session, k searchKind, ns string) ([][2]string, error) {
	key := fmt.Sprintf("%p|%s|%s", s, k.Ref.Resource, ns)
	searchMu.Lock()
	for kk, v := range searchCache { // drop stale lists (frees memory)
		if time.Since(v.at) > searchTTL {
			delete(searchCache, kk)
		}
	}
	if v, ok := searchCache[key]; ok {
		searchMu.Unlock()
		return v.items, nil
	}
	searchMu.Unlock()

	r := mc.Resource(k.Ref.GVR())
	var list *metav1.PartialObjectMetadataList
	var err error
	if k.Ref.Namespaced {
		list, err = r.Namespace(ns).List(ctx, metav1.ListOptions{})
	} else {
		list, err = r.List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, err
	}
	items := make([][2]string, 0, len(list.Items))
	for _, it := range list.Items {
		items = append(items, [2]string{it.Namespace, it.Name})
	}
	searchMu.Lock()
	searchCache[key] = nameList{at: time.Now(), items: items}
	searchMu.Unlock()
	return items, nil
}

// Search finds objects whose name contains q (case-insensitive) in the given
// namespaces (none = all). Kinds the user may not list are skipped.
func Search(ctx context.Context, s *cluster.Session, namespaces []string, q string, limit int) ([]SearchHit, error) {
	q = strings.ToLower(strings.TrimSpace(q))
	if len(q) < 2 {
		return []SearchHit{}, nil
	}
	mc, err := metadata.NewForConfig(s.Config)
	if err != nil {
		return nil, err
	}
	nss := namespaces
	if len(nss) == 0 {
		nss = []string{""} // all namespaces
	}
	type scored struct {
		hit   SearchHit
		score int
	}
	var (
		mu  sync.Mutex
		all []scored
		wg  sync.WaitGroup
	)
	for ki, k := range searchKinds {
		scopes := nss
		if !k.Ref.Namespaced {
			scopes = []string{""}
		}
		for _, ns := range scopes {
			wg.Add(1)
			go func(ki int, k searchKind, ns string) {
				defer wg.Done()
				items, err := listNames(ctx, mc, s, k, ns)
				if err != nil {
					return // forbidden or not served: skip this kind
				}
				var mine []scored
				for _, it := range items {
					name := strings.ToLower(it[1])
					i := strings.Index(name, q)
					if i < 0 {
						continue
					}
					sc := ki // earlier kinds first
					switch {
					case name == q:
						sc += 0
					case i == 0:
						sc += 100
					default:
						sc += 200
					}
					mine = append(mine, scored{SearchHit{Kind: k.Kind, Namespace: it[0], Name: it[1]}, sc})
				}
				mu.Lock()
				all = append(all, mine...)
				mu.Unlock()
			}(ki, k, ns)
		}
	}
	wg.Wait()
	slices.SortFunc(all, func(a, b scored) int {
		if a.score != b.score {
			return a.score - b.score
		}
		return strings.Compare(a.hit.Name, b.hit.Name)
	})
	out := make([]SearchHit, 0, min(limit, len(all)))
	for _, s := range all {
		if len(out) >= limit {
			break
		}
		out = append(out, s.hit)
	}
	return out, nil
}
