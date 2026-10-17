// Package events is the single bridge between backend goroutines and the UI.
package events

import (
	"context"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Emitter is the only way backend packages talk to the frontend. Keeping it an
// interface keeps domain packages free of Wails imports (unit-testable, and a
// future move to Wails v3 only touches this file).
type Emitter interface {
	Emit(topic string, payload any)
}

// Bus implements Emitter on top of Wails runtime events. The Wails context is
// only known at OnStartup, so it is bound late; emits before that are dropped.
type Bus struct {
	ctx atomic.Pointer[context.Context]
}

func NewBus() *Bus { return &Bus{} }

func (b *Bus) Bind(ctx context.Context) { b.ctx.Store(&ctx) }

func (b *Bus) Emit(topic string, payload any) {
	if c := b.ctx.Load(); c != nil {
		runtime.EventsEmit(*c, topic, payload)
	}
}

// Context returns the Wails app context (needed for native dialogs).
func (b *Bus) Context() context.Context {
	if c := b.ctx.Load(); c != nil {
		return *c
	}
	return context.Background()
}
