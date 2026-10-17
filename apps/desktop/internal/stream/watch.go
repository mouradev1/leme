package stream

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"leme/internal/platform/events"
	"leme/internal/resources"
)

// ResourceEvent is the payload of topic "watch:<subID>".
//
//	reset  -> new snapshot begins (columns + first page)
//	append -> more rows of the same snapshot (pagination)
//	synced -> snapshot complete; UI drops rows not seen since "reset"
//	delta  -> batched ADDED/MODIFIED/DELETED, coalesced per UID
//	error  -> transient error; the watcher keeps retrying with backoff
type ResourceEvent struct {
	Type    string             `json:"type"`
	Columns []resources.Column `json:"columns,omitempty"`
	Rows    []resources.Row    `json:"rows,omitempty"`
	Deltas  []Delta            `json:"deltas,omitempty"`
	Error   string             `json:"error,omitempty"`
}

type Delta struct {
	Type string        `json:"type"` // ADDED | MODIFIED | DELETED
	Row  resources.Row `json:"row"`
}

const pageSize = 500

var errGone = errors.New("resourceVersion expired")

// WatchTable is a list+watch loop (what an informer does) but stateless on
// our side: rows go straight to the UI, nothing is cached in Go memory.
func WatchTable(ctx context.Context, rc rest.Interface, ref resources.Ref, q resources.Query, emit func(ResourceEvent)) {
	backoff := time.Second
	for ctx.Err() == nil {
		rv, err := listAll(ctx, rc, ref, q, emit)
		if err == nil {
			backoff = time.Second
			err = watchFrom(ctx, rc, ref, q, rv, emit)
		}
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errGone) {
			continue // relist immediately
		}
		if err != nil {
			emit(ResourceEvent{Type: "error", Error: err.Error()})
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func listAll(ctx context.Context, rc rest.Interface, ref resources.Ref, q resources.Query, emit func(ResourceEvent)) (string, error) {
	cont, first := "", true
	for {
		p, err := resources.ListPage(ctx, rc, ref, q, cont, pageSize)
		if err != nil {
			return "", err
		}
		if first {
			emit(ResourceEvent{Type: "reset", Columns: p.Columns, Rows: p.Rows})
			first = false
		} else {
			emit(ResourceEvent{Type: "append", Rows: p.Rows})
		}
		if p.Continue == "" {
			emit(ResourceEvent{Type: "synced"})
			return p.RV, nil
		}
		cont = p.Continue
	}
}

func watchFrom(ctx context.Context, rc rest.Interface, ref resources.Ref, q resources.Query, rv string, emit func(ResourceEvent)) error {
	// 150ms coalescing: a rolling update on 300 pods produces thousands of
	// MODIFIED events; the UI only needs the latest state of each row.
	batch := events.NewBatcher(150*time.Millisecond, 1000, func(ds []Delta) {
		emit(ResourceEvent{Type: "delta", Deltas: coalesce(ds)})
	})
	defer batch.Flush()

	for {
		body, err := resources.OpenWatch(ctx, rc, ref, q, rv)
		if err != nil {
			if apierrors.IsGone(err) || apierrors.IsResourceExpired(err) {
				return errGone
			}
			return err
		}
		err = consume(ctx, body, &rv, batch)
		_ = body.Close()
		if err != nil {
			return err
		}
		// Server closed the watch at timeoutSeconds: resume from last rv.
	}
}

func consume(ctx context.Context, body io.Reader, rv *string, batch *events.Batcher[Delta]) error {
	dec := json.NewDecoder(body)
	for {
		var ev resources.WatchEvent
		if err := dec.Decode(&ev); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch ev.Type {
		case "ERROR":
			var st metav1.Status
			_ = json.Unmarshal(ev.Object, &st)
			if st.Code == 410 {
				return errGone
			}
			return &apierrors.StatusError{ErrStatus: st}
		case "BOOKMARK":
			continue
		}
		rows, err := resources.DecodeRows(ev.Object)
		if err != nil {
			return err
		}
		for _, r := range rows {
			batch.Add(Delta{Type: ev.Type, Row: r})
			if r.RV != "" {
				*rv = r.RV
			}
		}
	}
}

// coalesce keeps only the last delta per UID, preserving arrival order.
func coalesce(ds []Delta) []Delta {
	last := make(map[string]int, len(ds))
	for i, d := range ds {
		last[d.Row.UID] = i
	}
	out := make([]Delta, 0, len(last))
	for i, d := range ds {
		if last[d.Row.UID] == i {
			out = append(out, d)
		}
	}
	return out
}
