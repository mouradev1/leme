package terminal

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"leme/internal/cluster"
)

// Node shells run in a short-lived privileged pod pinned to the node, which
// enters the host's namespaces (PID 1) with nsenter. The pod is deleted when
// the shell ends; activeDeadlineSeconds is the last-resort cleanup.
const (
	nodeShellNamespace = "kube-system" // usually allowed to run privileged pods (Pod Security)
	nodeShellImage     = "docker.io/library/busybox:1.36"
	nodeShellLabel     = "leme.dev/node-shell"
	nodeShellMaxAge    = 8 * 60 * 60 // seconds
)

// nodeShellCommand runs inside the pod: switch to the host's mount, UTS, IPC,
// network and PID namespaces, then start the host's own shell.
var nodeShellCommand = []string{
	"nsenter", "-t", "1", "-m", "-u", "-i", "-n", "-p", "--",
	"sh", "-c", "if [ -x /bin/bash ]; then exec /bin/bash -l; else exec /bin/sh -l; fi",
}

func nodeShellPod(node string) *corev1.Pod {
	zero := int64(0)
	deadline := int64(nodeShellMaxAge)
	privileged := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "leme-node-shell-",
			Namespace:    nodeShellNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "leme",
				nodeShellLabel:                 "true",
			},
			Annotations: map[string]string{"leme.dev/node": node},
		},
		Spec: corev1.PodSpec{
			NodeName:                      node, // bypass the scheduler: exactly this node
			HostPID:                       true,
			HostNetwork:                   true,
			HostIPC:                       true,
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &zero,
			ActiveDeadlineSeconds:         &deadline,
			PriorityClassName:             "system-node-critical",
			Tolerations:                   []corev1.Toleration{{Operator: corev1.TolerationOpExists}}, // tainted / cordoned nodes too
			Containers: []corev1.Container{{
				Name:            "shell",
				Image:           nodeShellImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"sh", "-c", "while true; do sleep 3600; done"},
				SecurityContext: &corev1.SecurityContext{Privileged: &privileged},
			}},
		},
	}
}

// NodeShell opens a root shell on a node. Progress is written to the
// terminal while the helper pod starts.
func NodeShell(s *cluster.Session, node string, cols, rows uint16) Starter {
	return func(out func([]byte), done func(error)) (Session, error) {
		say := func(format string, a ...any) { out([]byte(fmt.Sprintf("\x1b[90m"+format+"\x1b[0m\r\n", a...))) }
		ctx := s.Ctx()

		say("Criando pod privilegiado temporário no node %s (%s)...", node, nodeShellNamespace)
		pod, err := s.Kube.CoreV1().Pods(nodeShellNamespace).Create(ctx, nodeShellPod(node), metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsForbidden(err) {
				return nil, fmt.Errorf("sem permissão para criar pods privilegiados em %s: %w", nodeShellNamespace, err)
			}
			return nil, err
		}
		var once sync.Once
		cleanup := func() {
			once.Do(func() {
				// Session context may already be cancelled; use a fresh one.
				dctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				_ = s.Kube.CoreV1().Pods(nodeShellNamespace).Delete(dctx, pod.Name, metav1.DeleteOptions{})
			})
		}

		if err := waitRunning(ctx, s.Kube, pod.Name, say); err != nil {
			cleanup()
			return nil, err
		}
		say("Conectado ao node %s como root. O pod %s é removido ao fechar este terminal.", node, pod.Name)

		inner, err := Exec(s, ExecTarget{Namespace: nodeShellNamespace, Pod: pod.Name, Container: "shell", Command: nodeShellCommand}, cols, rows)(
			out, func(e error) { cleanup(); done(e) })
		if err != nil {
			cleanup()
			return nil, err
		}
		return &nodeShellSession{Session: inner, cleanup: cleanup}, nil
	}
}

type nodeShellSession struct {
	Session
	cleanup func()
}

func (n *nodeShellSession) Close() error {
	err := n.Session.Close()
	n.cleanup()
	return err
}

func waitRunning(ctx context.Context, kube kubernetes.Interface, name string, say func(string, ...any)) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	lastReason := ""
	for {
		p, err := kube.CoreV1().Pods(nodeShellNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		switch p.Status.Phase {
		case corev1.PodRunning:
			return nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return fmt.Errorf("o pod auxiliar terminou (%s): %s", p.Status.Phase, p.Status.Message)
		}
		for _, cs := range p.Status.ContainerStatuses {
			if w := cs.State.Waiting; w != nil {
				if w.Reason != lastReason {
					lastReason = w.Reason
					say("Aguardando: %s", w.Reason)
				}
				if strings.Contains(w.Reason, "ImagePull") || w.Reason == "ErrImageNeverPull" || w.Reason == "InvalidImageName" {
					return fmt.Errorf("não foi possível baixar a imagem %s no node (o cluster tem acesso ao Docker Hub?): %s", nodeShellImage, w.Message)
				}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("o pod auxiliar não iniciou a tempo no node (%s)", lastReason)
		case <-time.After(time.Second):
		}
	}
}
