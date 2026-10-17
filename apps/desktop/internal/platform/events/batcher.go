package events

import (
	"sync"
	"time"
)

// Batcher coalesces high-frequency items (watch deltas, log lines, PTY bytes)
// into a single IPC message per interval. Crossing the Go<->WebView bridge is
// the dominant cost, so emitting 1 message with 500 lines instead of 500
// messages is what keeps the UI responsive on noisy clusters.
type Batcher[T any] struct {
	mu       sync.Mutex
	buf      []T
	max      int
	interval time.Duration
	timer    *time.Timer
	flush    func([]T)
}

func NewBatcher[T any](interval time.Duration, max int, flush func([]T)) *Batcher[T] {
	return &Batcher[T]{interval: interval, max: max, flush: flush, buf: make([]T, 0, max)}
}

func (b *Batcher[T]) Add(items ...T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, items...)
	if len(b.buf) >= b.max {
		b.flushLocked()
		return
	}
	if b.timer == nil {
		b.timer = time.AfterFunc(b.interval, b.Flush)
	}
}

// Flush emits whatever is buffered. Safe to call concurrently and repeatedly.
func (b *Batcher[T]) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked()
}

// flushLocked runs the callback under the lock on purpose: it preserves event
// ordering, and the callback (a Wails emit) is non-blocking.
func (b *Batcher[T]) flushLocked() {
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	if len(b.buf) == 0 {
		return
	}
	out := b.buf
	b.buf = make([]T, 0, b.max)
	b.flush(out)
}
