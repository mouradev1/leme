package resources

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"leme/internal/cluster"
)

// MaxLogStreams caps how many pod/container log streams one tab may open.
const MaxLogStreams = 30

type PodContainer struct {
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

type WorkloadPods struct {
	Targets []PodContainer `json:"targets"`
	Pods    int            `json:"pods"`  // pods matched by the selector
	Total   int            `json:"total"` // pod/container pairs before the cap
}

// PodSelector returns the label selector a workload uses to own its pods:
// spec.selector (LabelSelector) for Deployment/StatefulSet/DaemonSet/
// ReplicaSet/Job, or the plain map selector of a Service.
func PodSelector(obj *unstructured.Unstructured) (labels.Selector, error) {
	raw, found, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", "selector")
	if err != nil || !found || raw == nil {
		return nil, fmt.Errorf("%s %q não tem seletor de pods", obj.GetKind(), obj.GetName())
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("seletor inválido em %s %q", obj.GetKind(), obj.GetName())
	}
	if obj.GetKind() == "Service" {
		set := labels.Set{}
		for k, v := range m {
			if s, ok := v.(string); ok {
				set[k] = s
			}
		}
		if len(set) == 0 {
			return nil, fmt.Errorf("o service %q não tem seletor de pods", obj.GetName())
		}
		return labels.SelectorFromSet(set), nil
	}
	var ls metav1.LabelSelector
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &ls); err != nil {
		return nil, err
	}
	sel, err := metav1.LabelSelectorAsSelector(&ls)
	if err != nil {
		return nil, err
	}
	if sel.Empty() {
		return nil, fmt.Errorf("%s %q tem seletor vazio", obj.GetKind(), obj.GetName())
	}
	return sel, nil
}

// SelectorOf returns the pod label selector (as a string) of a workload or
// Service, so the UI can watch its pods live.
func SelectorOf(ctx context.Context, s *cluster.Session, ref Ref, ns, name string) (string, error) {
	obj, err := resourceClient(s, ref, ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	sel, err := PodSelector(obj)
	if err != nil {
		return "", err
	}
	return sel.String(), nil
}

// PodsOf lists the running/pending pods owned (by selector) by a workload and
// expands them into pod/container log targets, capped at MaxLogStreams.
func PodsOf(ctx context.Context, s *cluster.Session, ref Ref, ns, name string) (WorkloadPods, error) {
	var out WorkloadPods
	sel, err := SelectorOf(ctx, s, ref, ns, name)
	if err != nil {
		return out, err
	}
	pods, err := s.Kube.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return out, err
	}
	items := pods.Items
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	out.Pods = len(items)
	for _, p := range items {
		for _, c := range p.Spec.Containers {
			out.Total++
			if len(out.Targets) < MaxLogStreams {
				out.Targets = append(out.Targets, PodContainer{Pod: p.Name, Container: c.Name})
			}
		}
	}
	return out, nil
}
