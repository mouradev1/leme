package api

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"leme/internal/ops"
	"leme/internal/pods"
	"leme/internal/resources"
)

type ResourceAPI struct{ d *Deps }

func (a *ResourceAPI) Catalog(cluster string) ([]resources.Kind, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return nil, err
	}
	return resources.Catalog(s)
}

func (a *ResourceAPI) RefreshCatalog(cluster string) ([]resources.Kind, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return nil, err
	}
	resources.InvalidateCatalog(s)
	return resources.Catalog(s)
}

func (a *ResourceAPI) GetYAML(cluster string, ref resources.Ref, namespace, name string) (string, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return "", err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	return resources.GetYAML(ctx, s, ref, namespace, name)
}

// Apply with opts.DryRun=true is what the editor's "Diff" button calls.
func (a *ResourceAPI) Apply(cluster, yaml string, opts resources.ApplyOptions) (resources.ApplyResult, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return resources.ApplyResult{}, err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	return resources.Apply(ctx, s, yaml, opts)
}

func (a *ResourceAPI) Delete(cluster string, ref resources.Ref, namespace, name string, force bool) error {
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	return resources.Delete(ctx, s, ref, namespace, name, force)
}

func (a *ResourceAPI) Scale(cluster string, ref resources.Ref, namespace, name string, replicas int32) error {
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	return ops.Scale(ctx, s, ref, namespace, name, replicas)
}

func (a *ResourceAPI) Restart(cluster string, ref resources.Ref, namespace, name string) error {
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	return ops.Restart(ctx, s, ref, namespace, name)
}

func (a *ResourceAPI) Cordon(cluster, node string, cordon bool) error {
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	return ops.Cordon(ctx, s, node, cordon)
}

type DrainEvent struct {
	Type    string `json:"type"` // progress | done | error
	Message string `json:"message"`
}

// Drain can take minutes (PDB retries), so it runs as a subscription that
// streams progress on "drain:<subID>" and can be cancelled with Unsubscribe.
func (a *ResourceAPI) Drain(subID, cluster, node string, opts ops.DrainOptions) error {
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	topic := "drain:" + subID
	a.d.stream(s, subID, func(ctx context.Context) {
		err := ops.Drain(ctx, s, node, opts, func(line string) {
			a.d.Bus.Emit(topic, DrainEvent{Type: "progress", Message: line})
		})
		if err != nil {
			a.d.Bus.Emit(topic, DrainEvent{Type: "error", Message: err.Error()})
			return
		}
		a.d.Bus.Emit(topic, DrainEvent{Type: "done"})
	})
	return nil
}

type PodContainers struct {
	Containers     []string `json:"containers"`
	InitContainers []string `json:"initContainers"`
}

// PodContainers feeds the Logs/Shell pickers. Table rows carry metadata only,
// so this is fetched on demand when the user opens one of those actions.
func (a *ResourceAPI) PodContainers(cluster, namespace, pod string) (PodContainers, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return PodContainers{}, err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	p, err := s.Kube.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return PodContainers{}, err
	}
	var out PodContainers
	for _, c := range p.Spec.Containers {
		out.Containers = append(out.Containers, c.Name)
	}
	for _, c := range p.Spec.InitContainers {
		out.InitContainers = append(out.InitContainers, c.Name)
	}
	return out, nil
}

// GetData returns the text data of a ConfigMap or Secret (decoded) for the
// details panel. resource is "configmaps" or "secrets".
func (a *ResourceAPI) GetData(cluster, resource, namespace, name string) (resources.ObjectData, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return resources.ObjectData{}, err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	return resources.GetData(ctx, s, resource, namespace, name)
}

// SetData saves edited keys; returns the new resourceVersion.
func (a *ResourceAPI) SetData(cluster, resource, namespace, name string, data map[string]string, resourceVersion string) (string, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return "", err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	return resources.SetData(ctx, s, resource, namespace, name, data, resourceVersion)
}

// PodDetails returns the structured pod view plus the restart diagnosis.
func (a *ResourceAPI) PodDetails(cluster, namespace, name string) (pods.Details, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return pods.Details{}, err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	return pods.Get(ctx, s.Kube, namespace, name)
}
