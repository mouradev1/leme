package terminal

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNodeShellPodSpec(t *testing.T) {
	p := nodeShellPod("k8s-node-a")
	c := p.Spec.Containers[0]
	switch {
	case p.Spec.NodeName != "k8s-node-a":
		t.Fatal("pod must be pinned to the node")
	case !p.Spec.HostPID || !p.Spec.HostNetwork || !p.Spec.HostIPC:
		t.Fatal("host namespaces required for nsenter")
	case c.SecurityContext == nil || c.SecurityContext.Privileged == nil || !*c.SecurityContext.Privileged:
		t.Fatal("container must be privileged")
	case len(p.Spec.Tolerations) != 1 || p.Spec.Tolerations[0].Operator != corev1.TolerationOpExists:
		t.Fatal("must tolerate any taint (cordoned / tainted nodes)")
	case p.Spec.ActiveDeadlineSeconds == nil || *p.Spec.ActiveDeadlineSeconds != nodeShellMaxAge:
		t.Fatal("last-resort expiry missing")
	case p.Labels[nodeShellLabel] != "true" || p.Labels["app.kubernetes.io/managed-by"] != "leme":
		t.Fatal("pod must be labelled as Leme's")
	case p.Spec.RestartPolicy != corev1.RestartPolicyNever:
		t.Fatal("helper pod must not restart")
	}
	if strings.Join(nodeShellCommand[:3], " ") != "nsenter -t 1" {
		t.Fatalf("shell must enter PID 1's namespaces: %v", nodeShellCommand)
	}
}

func TestWaitRunningReportsImagePullErrors(t *testing.T) {
	kube := fake.NewClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "leme-node-shell-x", Namespace: nodeShellNamespace},
		Status: corev1.PodStatus{Phase: corev1.PodPending, ContainerStatuses: []corev1.ContainerStatus{{
			Name: "shell", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ErrImagePull", Message: "dial tcp: i/o timeout"}},
		}}},
	})
	err := waitRunning(context.Background(), kube, "leme-node-shell-x", func(string, ...any) {})
	if err == nil || !strings.Contains(err.Error(), "baixar a imagem") {
		t.Fatalf("want a friendly image pull error, got %v", err)
	}
}

type fakeSession struct{ closed atomic.Bool }

func (f *fakeSession) Write([]byte) error           { return nil }
func (f *fakeSession) Resize(uint16, uint16) error { return nil }
func (f *fakeSession) Close() error                 { f.closed.Store(true); return nil }

type nopBus struct{}

func (nopBus) Emit(string, any) {}

// Closing a tab while a node shell is still starting must not leak the
// session (and its privileged pod): it's closed as soon as it's ready.
func TestCloseWhileStarting(t *testing.T) {
	m := NewManager(nopBus{})
	sess := &fakeSession{}
	ready := make(chan struct{})
	opened := make(chan error, 1)
	go func() {
		opened <- m.Open("t1", func(func([]byte), func(error)) (Session, error) {
			<-ready // e.g. waiting for the helper pod
			return sess, nil
		})
	}()
	m.Close("t1") // user closed the tab meanwhile
	close(ready)
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	if !sess.closed.Load() {
		t.Fatal("session started after Close was not closed")
	}
	if _, err := m.get("t1"); err == nil {
		t.Fatal("closed session must not be registered")
	}
}
