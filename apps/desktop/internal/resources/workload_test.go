package resources

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func TestPodSelector(t *testing.T) {
	deploy := &unstructured.Unstructured{Object: map[string]any{
		"kind": "Deployment", "metadata": map[string]any{"name": "api"},
		"spec": map[string]any{"selector": map[string]any{
			"matchLabels":      map[string]any{"app": "api"},
			"matchExpressions": []any{map[string]any{"key": "tier", "operator": "In", "values": []any{"web", "edge"}}},
		}},
	}}
	sel, err := PodSelector(deploy)
	if err != nil {
		t.Fatal(err)
	}
	if !sel.Matches(labels.Set{"app": "api", "tier": "web"}) || sel.Matches(labels.Set{"app": "api", "tier": "db"}) {
		t.Fatalf("deployment selector wrong: %s", sel)
	}

	svc := &unstructured.Unstructured{Object: map[string]any{
		"kind": "Service", "metadata": map[string]any{"name": "web"},
		"spec": map[string]any{"selector": map[string]any{"app": "web"}},
	}}
	if sel, err := PodSelector(svc); err != nil || !sel.Matches(labels.Set{"app": "web"}) {
		t.Fatalf("service selector: %v %v", sel, err)
	}

	empty := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "ext"}, "spec": map[string]any{}}}
	if _, err := PodSelector(empty); err == nil {
		t.Fatal("service without selector must error (would match every pod)")
	}
}

// The selector travels to the UI as a string and comes back as a watch's
// labelSelector, so it must parse to the same selector.
func TestPodSelectorStringRoundTrip(t *testing.T) {
	sts := &unstructured.Unstructured{Object: map[string]any{
		"kind": "StatefulSet", "metadata": map[string]any{"name": "db"},
		"spec": map[string]any{"selector": map[string]any{
			"matchLabels":      map[string]any{"app": "db"},
			"matchExpressions": []any{map[string]any{"key": "canary", "operator": "DoesNotExist"}},
		}},
	}}
	sel, err := PodSelector(sts)
	if err != nil {
		t.Fatal(err)
	}
	back, err := labels.Parse(sel.String())
	if err != nil {
		t.Fatalf("parse %q: %v", sel.String(), err)
	}
	if !back.Matches(labels.Set{"app": "db"}) || back.Matches(labels.Set{"app": "db", "canary": "1"}) {
		t.Fatalf("round trip changed selector: %s", back)
	}
}
