package resources

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
}

// ApplyResult is what the editor shows after applying.
type ApplyResult struct {
	YAML string `json:"yaml"`
	// Warnings: e.g. the object is also managed by a deploy tool (Argo CD,
	// Flux, Helm, a CI pipeline) that may revert a manual edit on its next run.
	Warnings []string `json:"warnings"`
}

// Apply saves one or more YAML/JSON documents with "edit" semantics, like
// `kubectl edit`: an existing object is replaced by exactly what's in the
// editor (changed AND removed fields take effect), a new one is created.
//
// Why not server-side apply: SSA tracks an owner per field, so editing a field
// owned by another tool fails with a conflict, and removing such a field from
// the YAML is silently ignored. Neither is what "edit this YAML" means.
//
// Safety is kept:
//   - Validation is the API server's own (fieldValidation=Strict): unknown or
//     duplicated fields, CRD OpenAPI schemas and admission webhooks all run.
//   - The editor's diff = GetYAML (live) vs Apply(DryRun) (result), showing
//     defaulting/mutation exactly as the cluster would persist it.
//   - metadata.resourceVersion in the YAML is an optimistic lock: saving a
//     stale editor buffer fails instead of overwriting someone else's change.
func Apply(ctx context.Context, s *cluster.Session, doc string, o ApplyOptions) (ApplyResult, error) {
	dec := utilyaml.NewYAMLOrJSONDecoder(strings.NewReader(doc), 4096)
	var res ApplyResult
	var docs []string
	for {
		obj := &unstructured.Unstructured{}
		if err := dec.Decode(&obj.Object); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return res, fmt.Errorf("YAML inválido: %w", err)
		}
		if len(obj.Object) == 0 {
			continue // empty document between "---"
		}
		out, warnings, err := applyOne(ctx, s, obj, o)
		if err != nil {
			return res, fmt.Errorf("%s %q: %w", obj.GetKind(), obj.GetName(), friendly(err))
		}
		docs = append(docs, out)
		res.Warnings = append(res.Warnings, warnings...)
	}
	res.YAML = strings.Join(docs, "---\n")
	return res, nil
}

func applyOne(ctx context.Context, s *cluster.Session, obj *unstructured.Unstructured, o ApplyOptions) (string, []string, error) {
	gvk := obj.GroupVersionKind()
	mapping, err := s.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return "", nil, err
	}
	var ri dynamic.ResourceInterface = s.Dynamic.Resource(mapping.Resource)
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = s.DefaultNamespace()
			obj.SetNamespace(ns)
		}
		ri = s.Dynamic.Resource(mapping.Resource).Namespace(ns)
	}
	// managedFields are hidden in the editor; nil means "leave as is" on update.
	obj.SetManagedFields(nil)
	unstructured.RemoveNestedField(obj.Object, "status") // status is owned by controllers

	var dryRun []string
	if o.DryRun {
		dryRun = []string{metav1.DryRunAll}
	}

	current, err := ri.Get(ctx, obj.GetName(), metav1.GetOptions{})
	var out *unstructured.Unstructured
	switch {
	case apierrors.IsNotFound(err):
		obj.SetResourceVersion("")
		out, err = ri.Create(ctx, obj, metav1.CreateOptions{FieldManager: FieldManager, FieldValidation: "Strict", DryRun: dryRun})
	case err != nil:
		return "", nil, err
	default:
		out, err = ri.Update(ctx, obj, metav1.UpdateOptions{FieldManager: FieldManager, FieldValidation: "Strict", DryRun: dryRun})
	}
	if err != nil {
		return "", nil, err
	}
	y, err := toYAML(out)
	if err != nil {
		return "", nil, err
	}
	var warnings []string
	if current != nil && err == nil {
		if tools := externalManagers(current); len(tools) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"%s %q também é gerenciado por: %s. Se for uma ferramenta de deploy (pipeline de CI/CD, Argo CD, Flux, Helm), o próximo deploy dela pode desfazer esta alteração. Para mudar de forma permanente, altere também na origem (repositório, chart ou pipeline).",
				obj.GetKind(), obj.GetName(), strings.Join(tools, ", ")))
		}
	}
	return y, warnings, nil
}

// externalManagers lists field managers that look like deploy tools, i.e.
// not Kubernetes' own controllers, kubectl, or Leme itself.
func externalManagers(obj *unstructured.Unstructured) []string {
	internal := []string{"kube-controller-manager", "kube-scheduler", "kubelet", "kube-apiserver", "kubectl", FieldManager}
	seen := map[string]bool{}
	var out []string
	for _, mf := range obj.GetManagedFields() {
		m := mf.Manager
		if m == "" || seen[m] || mf.Subresource == "status" {
			continue
		}
		seen[m] = true
		isInternal := false
		for _, p := range internal {
			if strings.HasPrefix(m, p) {
				isInternal = true
				break
			}
		}
		if !isInternal {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

// friendly turns the API errors users hit while editing into plain language.
func friendly(err error) error {
	switch {
	case apierrors.IsConflict(err):
		return fmt.Errorf("o objeto foi alterado no cluster depois que você abriu o editor. Clique em Recarregar para ver a versão atual e refaça a alteração (assim nada de outra pessoa é sobrescrito)")
	case apierrors.IsInvalid(err), apierrors.IsBadRequest(err):
		return fmt.Errorf("o cluster recusou o YAML: %w", err)
	case apierrors.IsForbidden(err):
		return fmt.Errorf("sem permissão para alterar este objeto: %w", err)
	}
	return err
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
