package terminal

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"leme/internal/cluster"
)

type ExecTarget struct {
	Namespace string   `json:"namespace"`
	Pod       string   `json:"pod"`
	Container string   `json:"container"`
	Command   []string `json:"command"` // empty = best available shell
}

// Prefer bash, fall back to sh: distroless/alpine images often lack bash.
var defaultShell = []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh"}

// Exec opens an interactive TTY in a container (`kubectl exec -it`). The
// remote side allocates the PTY; we provide stdin, stdout and a resize queue.
// Uses the WebSocket executor (K8s >= 1.30) with fallback to SPDY, exactly
// like kubectl does.
func Exec(s *cluster.Session, t ExecTarget, cols, rows uint16) Starter {
	return func(out func([]byte), done func(error)) (Session, error) {
		cmd := t.Command
		if len(cmd) == 0 {
			cmd = defaultShell
		}
		req := s.Kube.CoreV1().RESTClient().Post().
			Resource("pods").Namespace(t.Namespace).Name(t.Pod).SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: t.Container, Command: cmd,
				Stdin: true, Stdout: true, Stderr: false, TTY: true, // TTY merges stderr into stdout
			}, scheme.ParameterCodec)

		spdy, err := remotecommand.NewSPDYExecutor(s.Config, "POST", req.URL())
		if err != nil {
			return nil, err
		}
		ws, err := remotecommand.NewWebSocketExecutor(s.Config, "GET", req.URL().String())
		if err != nil {
			return nil, err
		}
		exec, err := remotecommand.NewFallbackExecutor(ws, spdy, func(err error) bool {
			return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
		})
		if err != nil {
			return nil, err
		}

		ctx, cancel := context.WithCancel(s.Ctx()) // dies with the cluster session
		pr, pw := io.Pipe()
		es := &execSession{ctx: ctx, cancel: cancel, stdin: pw, sizes: make(chan remotecommand.TerminalSize, 1)}
		es.sizes <- remotecommand.TerminalSize{Width: cols, Height: rows}

		go func() {
			err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
				Stdin: pr, Stdout: writerFunc(out), Tty: true, TerminalSizeQueue: es,
			})
			_ = pr.Close()
			cancel()
			if ctx.Err() != nil {
				err = nil // closed by the user
			}
			done(err)
		}()
		return es, nil
	}
}

type execSession struct {
	ctx    context.Context
	cancel context.CancelFunc
	stdin  *io.PipeWriter
	sizes  chan remotecommand.TerminalSize
}

func (e *execSession) Write(p []byte) error { _, err := e.stdin.Write(p); return err }

// Resize keeps only the latest size: dragging a split pane fires dozens of
// resizes and only the final one matters.
func (e *execSession) Resize(cols, rows uint16) error {
	select {
	case <-e.sizes:
	default:
	}
	select { // never block: the session may already be gone
	case e.sizes <- remotecommand.TerminalSize{Width: cols, Height: rows}:
	default:
	}
	return nil
}

// Next implements remotecommand.TerminalSizeQueue. Returning nil when the
// session ends stops client-go's resize goroutine (otherwise it leaks).
func (e *execSession) Next() *remotecommand.TerminalSize {
	select {
	case sz := <-e.sizes:
		return &sz
	case <-e.ctx.Done():
		return nil
	}
}

func (e *execSession) Close() error {
	e.cancel()
	return e.stdin.Close()
}

type writerFunc func([]byte)

func (f writerFunc) Write(p []byte) (int, error) { f(p); return len(p), nil }
