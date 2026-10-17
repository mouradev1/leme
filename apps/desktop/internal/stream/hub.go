// Package stream runs long-lived producers (watches, logs, metrics polling)
// whose output is pushed to the UI as Wails events.
package stream

import (
	"context"
	"sync"
)

// Hub tracks subscriptions by an ID chosen by the frontend. Each subscription
// is a goroutine bound to a context: when a Vue component unmounts it calls
// Stop(id), and when a cluster disconnects its session context cancels every
// subscription under it. Nothing streams unless a view is looking at it,
// which is the main reason memory stays flat (vs. tools that cache whole clusters).
type Hub struct {
	mu   sync.Mutex
	subs map[string]*sub
}

type sub struct{ cancel context.CancelFunc }

func NewHub() *Hub { return &Hub{subs: map[string]*sub{}} }

// Start runs fn in a goroutine. Re-using an ID replaces the old subscription
// (e.g. the user changed namespace filter on the same table).
func (h *Hub) Start(parent context.Context, id string, fn func(ctx context.Context)) {
	ctx, cancel := context.WithCancel(parent)
	me := &sub{cancel: cancel}

	h.mu.Lock()
	if old := h.subs[id]; old != nil {
		old.cancel()
	}
	h.subs[id] = me
	h.mu.Unlock()

	go func() {
		defer func() {
			cancel()
			h.mu.Lock()
			if h.subs[id] == me { // don't remove a newer subscription with the same ID
				delete(h.subs, id)
			}
			h.mu.Unlock()
		}()
		fn(ctx)
	}()
}

func (h *Hub) Stop(id string) {
	h.mu.Lock()
	if s := h.subs[id]; s != nil {
		s.cancel()
		delete(h.subs, id)
	}
	h.mu.Unlock()
}

func (h *Hub) StopAll() {
	h.mu.Lock()
	for id, s := range h.subs {
		s.cancel()
		delete(h.subs, id)
	}
	h.mu.Unlock()
}

func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
