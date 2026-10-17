// Package ops implements the quick actions with the same semantics kubectl
// uses, so behaviour matches what operators expect from the CLI.
package ops

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/drain"

	"leme/internal/cluster"
	"leme/internal/resources"
)

// Scale goes through the /scale subresource, so it works for Deployments,
// StatefulSets, ReplicaSets and any CRD that declares a scale subresource.
func Scale(ctx context.Context, s *cluster.Session, ref resources.Ref, ns, name string, replicas int32) error {
	patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)
	_, err := s.Dynamic.Resource(ref.GVR()).Namespace(ns).
		Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{FieldManager: resources.FieldManager}, "scale")
	return err
}

// Restart == `kubectl rollout restart`: bump a pod-template annotation.
func Restart(ctx context.Context, s *cluster.Session, ref resources.Ref, ns, name string) error {
	switch ref.Group + "/" + ref.Resource {
	case "apps/deployments", "apps/statefulsets", "apps/daemonsets":
	default:
		return fmt.Errorf("restart is not supported for %s", ref.Resource)
	}
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`,
		time.Now().Format(time.RFC3339))
	_, err := s.Dynamic.Resource(ref.GVR()).Namespace(ns).
		Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{FieldManager: resources.FieldManager})
	return err
}

func Cordon(ctx context.Context, s *cluster.Session, node string, cordon bool) error {
	n, err := s.Kube.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return err
	}
	return drain.RunCordonOrUncordon(&drain.Helper{Ctx: ctx, Client: s.Kube}, n, cordon)
}

type DrainOptions struct {
	Force              bool `json:"force"`              // delete pods without a controller
	DeleteEmptyDirData bool `json:"deleteEmptyDirData"` // allow losing emptyDir volumes
	GracePeriodSeconds int  `json:"gracePeriodSeconds"` // -1 = pod's own value
	TimeoutSeconds     int  `json:"timeoutSeconds"`     // 0 = wait forever
	DisableEviction    bool `json:"disableEviction"`    // bypass PodDisruptionBudgets (dangerous)
}

// Drain uses kubectl's own drain library (evictions + PDB retries), streaming
// its human-readable progress line by line to progress().
func Drain(ctx context.Context, s *cluster.Session, node string, o DrainOptions, progress func(string)) error {
	w := &lineWriter{fn: progress}
	h := &drain.Helper{
		Ctx: ctx, Client: s.Kube,
		Force: o.Force, DeleteEmptyDirData: o.DeleteEmptyDirData, DisableEviction: o.DisableEviction,
		GracePeriodSeconds:  o.GracePeriodSeconds,
		Timeout:             time.Duration(o.TimeoutSeconds) * time.Second,
		IgnoreAllDaemonSets: true,
		Out:                 w, ErrOut: w,
	}
	n, err := s.Kube.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if err := drain.RunCordonOrUncordon(h, n, true); err != nil {
		return err
	}
	return drain.RunNodeDrain(h, node)
}

type lineWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
	fn  func(string)
}

var _ io.Writer = (*lineWriter)(nil)

func (l *lineWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Write(p)
	for {
		line, err := l.buf.ReadString('\n')
		if err != nil {
			l.buf.WriteString(line) // incomplete line: keep for next write
			return len(p), nil
		}
		l.fn(line[:len(line)-1])
	}
}
