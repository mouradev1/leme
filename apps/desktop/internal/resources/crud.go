package resources

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"

	"leme/internal/cluster"
)

const FieldManager = "leme"

// GetYAML fetches the full object on demand (only when the editor opens).
func GetYAML(ctx context.Context, s *cluster.Session, ref Ref, ns, name string) (string, error) {
	obj, err := resourceClient(s, ref, ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return toYAML(obj)
}

type ApplyOptions struct {
	DryRun bool `json:"dryRun"` // server-side dry run: validates + returns the would-be object
	Force  bool `json:"force"`  // take ownership of fields owned by other managers (kubectl, helm...)
}

// Apply server-side-applies one or more YAML/JSON documents.
//
//   - Validation is the API server's own (fieldValidation=Strict): unknown or
//     duplicated fields, CRD OpenAPI schemas and admission webhooks all run.
//   - The editor's diff view = GetYAML (live) vs Apply(DryRun) (result), which
//     shows defaulting/mutation exactly as the cluster would persist it.
//   - A metadata.resourceVersion left in the YAML acts as an optimistic lock,
//     so saving a stale editor buffer fails with a Conflict instead of
//     silently overwriting someone else's change.
func Apply(ctx context.Context, s *cluster.Session, doc string, o ApplyOptions) (string, error) {
	dec := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(doc), 4096)
	var results []string
	for {
		obj := &unstructured.Unstructured{}
		if err := dec.Decode(&obj.Object); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", fmt.Errorf("parse yaml: %w", err)
		}
		if len(obj.Object) == 0 {
			continue // empty document between "---"
		}
		out, err := applyOne(ctx, s, obj, o)
		if err != nil {
			return "", fmt.Errorf("%s %q: %w", obj.GetKind(), obj.GetName(), err)
		}
		results = append(results, out)
	}
	return strings.Join(results, "---\n"), nil
}

func applyOne(ctx context.Context, s *cluster.Session, obj *unstructured.Unstructured, o ApplyOptions) (string, error) {
	gvk := obj.GroupVersionKind()
	mapping, err := s.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return "", err
	}
	var ri dynamic.ResourceInterface = s.Dynamic.Resource(mapping.Resource)
	if mapping.Scope.Name() == "namespace" {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = s.DefaultNamespace()
			obj.SetNamespace(ns)
		}
		ri = s.Dynamic.Resource(mapping.Resource).Namespace(ns)
	}
	obj.SetManagedFields(nil) // SSA rejects payloads that carry managedFields
	unstructured.RemoveNestedField(obj.Object, "status")

	body, err := obj.MarshalJSON()
	if err != nil {
		return "", err
	}
	opts := metav1.PatchOptions{FieldManager: FieldManager, Force: &o.Force, FieldValidation: "Strict"}
	if o.DryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	out, err := ri.Patch(ctx, obj.GetName(), types.ApplyPatchType, body, opts)
	if err != nil {
		return "", err
	}
	return toYAML(out)
}

func Delete(ctx context.Context, s *cluster.Session, ref Ref, ns, name string, force bool) error {
	bg := metav1.DeletePropagationBackground
	opts := metav1.DeleteOptions{PropagationPolicy: &bg}
	if force {
		zero := int64(0)
		opts.GracePeriodSeconds = &zero
	}
	return resourceClient(s, ref, ns).Delete(ctx, name, opts)
}

func resourceClient(s *cluster.Session, ref Ref, ns string) dynamic.ResourceInterface {
	if ref.Namespaced {
		return s.Dynamic.Resource(ref.GVR()).Namespace(ns)
	}
	return s.Dynamic.Resource(ref.GVR())
}

func toYAML(obj *unstructured.Unstructured) (string, error) {
	obj.SetManagedFields(nil) // noise; hundreds of lines on busy objects
	b, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(b)) + "\n", nil
}
