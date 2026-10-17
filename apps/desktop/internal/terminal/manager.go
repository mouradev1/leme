// Package terminal multiplexes interactive sessions (container exec and local
// PTY shells) over Wails events. Output is binary-safe: bytes are batched per
// frame and base64-encoded, so multi-byte UTF-8 split across reads, escape
// sequences and raw binary never get mangled by JSON string encoding.
package terminal

import (
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"leme/internal/platform/events"
)

type Session interface {
	Write(p []byte) error
	Resize(cols, rows uint16) error
	Close() error
}

// Starter launches a session. It must call out for every output chunk (the
// slice may be reused after return) and done exactly once when it ends.
type Starter func(out func([]byte), done func(error)) (Session, error)

type Manager struct {
	bus      events.Emitter
	mu       sync.Mutex
	sessions map[string]Session
}

func NewManager(bus events.Emitter) *Manager {
	return &Manager{bus: bus, sessions: map[string]Session{}}
}

// Open starts a session; the UI listens on "term:<id>:data" (base64 chunks)
// and "term:<id>:exit" (string reason, empty on clean exit).
func (m *Manager) Open(id string, start Starter) error {
	m.mu.Lock()
	if _, dup := m.sessions[id]; dup {
		m.mu.Unlock()
		return fmt.Errorf("terminal %q already open", id)
	}
	m.mu.Unlock()

	// ~1 frame of coalescing: `cat bigfile` stays smooth, keystroke echo
	// latency stays imperceptible.
	batch := events.NewBatcher(8*time.Millisecond, 256, func(chunks [][]byte) {
		n := 0
		for _, c := range chunks {
			n += len(c)
		}
		buf := make([]byte, 0, n)
		for _, c := range chunks {
			buf = append(buf, c...)
		}
		m.bus.Emit("term:"+id+":data", base64.StdEncoding.EncodeToString(buf))
	})
	out := func(p []byte) { batch.Add(append([]byte(nil), p...)) }
	done := func(err error) {
		batch.Flush()
		reason := ""
		if err != nil {
			reason = err.Error()
		}
		m.bus.Emit("term:"+id+":exit", reason)
		m.mu.Lock()
		delete(m.sessions, id)
		m.mu.Unlock()
	}

	s, err := start(out, done)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return nil
}

func (m *Manager) get(id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return nil, fmt.Errorf("terminal %q not found", id)
	}
	return s, nil
}

func (m *Manager) Write(id string, data []byte) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return s.Write(data)
}

func (m *Manager) Resize(id string, cols, rows uint16) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	return s.Resize(cols, rows)
}

func (m *Manager) Close(id string) {
	if s, err := m.get(id); err == nil {
		_ = s.Close()
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	all := make([]Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.mu.Unlock()
	for _, s := range all {
		_ = s.Close()
	}
}
