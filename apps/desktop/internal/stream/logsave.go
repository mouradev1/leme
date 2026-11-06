package stream

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// WriteLogs copies the logs of every target to w, one target after the other,
// streaming straight from the API (nothing is held in memory). Follow is
// ignored: a download is a snapshot. With several targets each line is tagged
// "[pod/container] ". Returns the number of lines written.
func WriteLogs(ctx context.Context, kube kubernetes.Interface, o LogOptions, w io.Writer) (int, error) {
	bw := bufio.NewWriterSize(w, 64<<10)
	n := 0
	var errs []error
	for _, t := range o.Targets {
		prefix := ""
		if len(o.Targets) > 1 {
			prefix = "[" + t.Pod + "/" + t.Container + "] "
		}
		c, err := copyLogs(ctx, kube, o, t, prefix, bw)
		n += c
		if err != nil {
			if ctx.Err() != nil {
				return n, ctx.Err()
			}
			errs = append(errs, errors.New(t.Pod+"/"+t.Container+": "+err.Error()))
		}
	}
	if err := bw.Flush(); err != nil {
		return n, err
	}
	return n, errors.Join(errs...)
}

func copyLogs(ctx context.Context, kube kubernetes.Interface, o LogOptions, t LogTarget, prefix string, w *bufio.Writer) (int, error) {
	opts := &corev1.PodLogOptions{Container: t.Container, Timestamps: o.Timestamps, Previous: o.Previous}
	if o.TailLines > 0 {
		opts.TailLines = &o.TailLines
	}
	if o.SinceSeconds > 0 {
		opts.SinceSeconds = &o.SinceSeconds
	}
	rc, err := kube.CoreV1().Pods(o.Namespace).GetLogs(t.Pod, opts).Stream(ctx)
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	r := bufio.NewReaderSize(rc, 32<<10)
	n := 0
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			if line[len(line)-1] != '\n' {
				line += "\n"
			}
			if _, werr := w.WriteString(prefix + line); werr != nil {
				return n, werr
			}
			n++
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return n, nil
			}
			return n, err
		}
	}
}

// CreatePrivate creates (or truncates) a file only the current user can read:
// logs often carry tokens, emails and other sensitive data.
func CreatePrivate(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	_ = f.Chmod(0o600) // an existing file keeps its old mode otherwise
	return f, nil
}
