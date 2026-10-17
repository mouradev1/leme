package cluster

import (
	"context"
	"testing"
	"time"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func fakeSession(lastUsed time.Time) *Session {
	s := &Session{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.lastUsed.Store(lastUsed.UnixNano())
	return s
}

func TestReapIdle(t *testing.T) {
	m := &Manager{bus: nopBus{}, sessions: map[string]*Session{}}
	m.raw = clientcmdapi.NewConfig() // in-memory; never reads the real kubeconfig

	old := time.Now().Add(-time.Hour)
	idle := fakeSession(old)
	streaming := fakeSession(old)
	streaming.Acquire()
	streaming.lastUsed.Store(old.UnixNano()) // still "old", but has a live stream
	recent := fakeSession(time.Now())
	m.sessions = map[string]*Session{"idle": idle, "streaming": streaming, "recent": recent}

	closed := m.ReapIdle(10 * time.Minute)
	if len(closed) != 1 || closed[0] != "idle" {
		t.Fatalf("closed = %v, want [idle]", closed)
	}
	if idle.Ctx().Err() == nil {
		t.Fatal("idle session context not cancelled")
	}
	if _, ok := m.sessions["streaming"]; !ok {
		t.Fatal("session with an open stream must never be reaped")
	}
	streaming.Release()
	streaming.lastUsed.Store(old.UnixNano())
	if closed := m.ReapIdle(10 * time.Minute); len(closed) != 1 || closed[0] != "streaming" {
		t.Fatalf("after stream closed, closed = %v", closed)
	}
}
