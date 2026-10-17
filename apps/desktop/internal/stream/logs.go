package stream

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"leme/internal/platform/events"
)

type LogTarget struct {
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

// LogOptions supports three modes: one container, all containers of a pod, or
// all pods of a workload (the frontend expands the selector into targets).
type LogOptions struct {
	Namespace    string      `json:"namespace"`
	Targets      []LogTarget `json:"targets"`
	TailLines    int64       `json:"tailLines"`
	SinceSeconds int64       `json:"sinceSeconds"`
	Timestamps   bool        `json:"timestamps"`
	Previous     bool        `json:"previous"`
	Follow       bool        `json:"follow"`
}

type LogLine struct {
	Pod       string `json:"p"`
	Container string `json:"c"`
	Text      string `json:"t"`
}

// LogEvent is the payload of topic "logs:<subID>".
type LogEvent struct {
	Type  string    `json:"type"` // lines | error | end
	Lines []LogLine `json:"lines,omitempty"`
	Error string    `json:"error,omitempty"`
}

const maxLine = 64 << 10 // truncate pathological single-line JSON dumps

// StreamLogs tails every target concurrently and multiplexes into one batched
// event stream. Search/filtering happens in the UI over a bounded ring buffer.
func StreamLogs(ctx context.Context, kube kubernetes.Interface, o LogOptions, emit func(LogEvent)) {
	batch := events.NewBatcher(50*time.Millisecond, 500, func(ls []LogLine) {
		emit(LogEvent{Type: "lines", Lines: ls})
	})

	var wg sync.WaitGroup
	for _, t := range o.Targets {
		wg.Add(1)
		go func(t LogTarget) {
			defer wg.Done()
			if err := tail(ctx, kube, o, t, batch); err != nil && ctx.Err() == nil {
				batch.Flush()
				emit(LogEvent{Type: "error", Error: t.Pod + "/" + t.Container + ": " + err.Error()})
			}
		}(t)
	}
	wg.Wait()
	batch.Flush()
	if ctx.Err() == nil {
		emit(LogEvent{Type: "end"})
	}
}

func tail(ctx context.Context, kube kubernetes.Interface, o LogOptions, t LogTarget, batch *events.Batcher[LogLine]) error {
	opts := &corev1.PodLogOptions{
		Container: t.Container, Follow: o.Follow, Timestamps: o.Timestamps, Previous: o.Previous,
	}
	if o.TailLines > 0 {
		opts.TailLines = &o.TailLines
	}
	if o.SinceSeconds > 0 {
		opts.SinceSeconds = &o.SinceSeconds
	}
	rc, err := kube.CoreV1().Pods(o.Namespace).GetLogs(t.Pod, opts).Stream(ctx)
	if err != nil {
		return err
	}
	defer rc.Close()

	r := bufio.NewReaderSize(rc, 32<<10)
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			if len(line) > maxLine {
				line = line[:maxLine] + " …[truncated]"
			}
			batch.Add(LogLine{Pod: t.Pod, Container: t.Container, Text: line})
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}
