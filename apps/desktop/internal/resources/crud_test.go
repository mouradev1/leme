package resources

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"leme/internal/cluster"
)

var deployGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

type resettable struct{ meta.RESTMapper }

func (resettable) Reset() {}

func fakeSession(objs ...runtime.Object) *cluster.Session {
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{deployGVR: "DeploymentList"}, objs...)
	m := meta.NewDefaultRESTMapper(nil)
	m.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
	return &cluster.Session{Dynamic: dyn, Mapper: resettable{m}}
}

func deployment() *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{
			"name": "api", "namespace": "shop", "resourceVersion": "7",
			"labels": map[string]any{"app": "api", "team": "payments"},
		},
		"spec": map[string]any{"replicas": int64(2)},
	}}
	u.SetManagedFields([]metav1.ManagedFieldsEntry{
		{Manager: "node-fetch", Operation: metav1.ManagedFieldsOperationApply},
		{Manager: "kube-controller-manager", Operation: metav1.ManagedFieldsOperationUpdate},
		{Manager: "kubectl-edit", Operation: metav1.ManagedFieldsOperationUpdate},
	})
	return u
}

// The scenario from the field: a field owned by another tool ("node-fetch").
// Editing it and removing another field must both take effect.
func TestApplyEditsFieldsOwnedByOtherTools(t *testing.T) {
	s := fakeSession(deployment())
	edited := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: shop
  resourceVersion: "7"
  labels:
    app: api
spec:
  replicas: 5
`
	res, err := Apply(context.Background(), s, edited, ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Dynamic.Resource(deployGVR).Namespace("shop").Get(context.Background(), "api", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Decoded YAML numbers are float64 in unstructured maps; the API server
	// receives them as JSON, so only the numeric value matters here.
	if r, _, _ := unstructured.NestedFieldNoCopy(got.Object, "spec", "replicas"); fmt.Sprint(r) != "5" {
		t.Fatalf("replicas = %v, want 5 (edit not applied)", r)
	}
	if _, ok := got.GetLabels()["team"]; ok {
		t.Fatal("label removed in the editor is still there (removal ignored)")
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "node-fetch") {
		t.Fatalf("expected a warning naming node-fetch, got %v", res.Warnings)
	}
	for _, internal := range []string{"kube-controller-manager", "kubectl-edit"} {
		if strings.Contains(res.Warnings[0], internal) {
			t.Fatalf("warning should not list %s: %s", internal, res.Warnings[0])
		}
	}
	if !strings.Contains(res.YAML, "replicas: 5") {
		t.Fatalf("result YAML not returned: %s", res.YAML)
	}
}

func TestApplyCreatesNewObjects(t *testing.T) {
	s := fakeSession()
	res, err := Apply(context.Background(), s, "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: shop\nspec:\n  replicas: 1\n", ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("new object should have no warnings: %v", res.Warnings)
	}
	if _, err := s.Dynamic.Resource(deployGVR).Namespace("shop").Get(context.Background(), "web", metav1.GetOptions{}); err != nil {
		t.Fatalf("object not created: %v", err)
	}
}

func TestApplyRejectsInvalidYAML(t *testing.T) {
	_, err := Apply(context.Background(), fakeSession(), "kind: [unclosed", ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "YAML inválido") {
		t.Fatalf("want friendly YAML error, got %v", err)
	}
}
