// Package cluster owns kubeconfig parsing and one lazily-created Session per
// context. Sessions live side by side, so switching clusters in the UI is just
// a different key here: no restart, no global "current context" mutation.
package cluster

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sync/singleflight"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"leme/internal/platform/events"
)

const TopicContextsChanged = "clusters:changed"

type Manager struct {
	bus events.Emitter

	mu       sync.RWMutex
	rules    *clientcmd.ClientConfigLoadingRules // rebuilt (never mutated) when paths change
	extra    []string                            // user-added kubeconfig files, beyond $KUBECONFIG / ~/.kube/config
	raw      *clientcmdapi.Config
	sessions map[string]*Session
	connect  singleflight.Group

	watcher *fsnotify.Watcher
}

// NewManager merges $KUBECONFIG (or ~/.kube/config) plus extraPaths exactly
// like kubectl does. Exec providers, auth-provider plugins (oidc), mTLS client
// certs, tokens and proxies are all handled by client-go's clientcmd.
func NewManager(bus events.Emitter, extraPaths ...string) (*Manager, error) {
	m := &Manager{bus: bus, sessions: map[string]*Session{}}
	for _, p := range extraPaths {
		if abs, err := normalize(p); err == nil && !slices.Contains(m.extra, abs) {
			m.extra = append(m.extra, abs)
		}
	}
	m.rules = newRules(m.extra)
	if err := m.Reload(); err != nil {
		return nil, err
	}
	m.watchFiles()
	return m, nil
}

// Reload re-reads every kubeconfig file. Sessions whose context, cluster or
// user definition changed are closed; untouched ones keep their watches alive.
func (m *Manager) Reload() error {
	m.mu.RLock()
	rules := m.rules
	m.mu.RUnlock()
	// Missing files are skipped by clientcmd, so a deleted/unmounted file
	// just makes its contexts disappear instead of breaking everything.
	raw, err := rules.Load()
	if err != nil {
		return fmt.Errorf("load kubeconfig: %w", err)
	}
	m.mu.Lock()
	first := m.raw == nil
	m.raw = raw
	for name, s := range m.sessions {
		if fingerprint(raw, name) != s.fingerprint {
			s.Close()
			delete(m.sessions, name)
		}
	}
	m.mu.Unlock()
	if !first {
		m.bus.Emit(TopicContextsChanged, m.Contexts())
	}
	return nil
}

func (m *Manager) Contexts() []ContextInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ContextInfo, 0, len(m.raw.Contexts))
	for name, c := range m.raw.Contexts {
		info := ContextInfo{
			Name: name, Cluster: c.Cluster, User: c.AuthInfo, Namespace: c.Namespace,
			Source: c.LocationOfOrigin, Current: name == m.raw.CurrentContext,
		}
		if cl := m.raw.Clusters[c.Cluster]; cl != nil {
			info.Server = cl.Server
		}
		if ai := m.raw.AuthInfos[c.AuthInfo]; ai != nil {
			info.Auth = authKind(ai)
		}
		_, info.Connected = m.sessions[name]
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Session returns the live session for a context, connecting on first use.
// Concurrent callers for the same context share one connection attempt.
func (m *Manager) Session(ctx context.Context, name string) (*Session, error) {
	m.mu.RLock()
	s := m.sessions[name]
	m.mu.RUnlock()
	if s != nil {
		return s, nil
	}

	v, err, _ := m.connect.Do(name, func() (any, error) {
		m.mu.RLock()
		raw, rules := m.raw, m.rules
		m.mu.RUnlock()
		if _, ok := raw.Contexts[name]; !ok {
			return nil, fmt.Errorf("context %q not found in kubeconfig", name)
		}
		// Passing the loading rules as ConfigAccess lets auth-provider plugins
		// persist refreshed tokens back into the right kubeconfig file.
		cc := clientcmd.NewNonInteractiveClientConfig(*raw, name, &clientcmd.ConfigOverrides{}, rules)
		s, err := newSession(ctx, name, cc)
		if err != nil {
			return nil, err
		}
		s.fingerprint = fingerprint(raw, name)
		m.mu.Lock()
		m.sessions[name] = s
		m.mu.Unlock()
		return s, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Session), nil
}

// ReapIdle closes sessions that have nothing streaming and haven't been used
// for maxIdle: their HTTP/2 connections, discovery cache and client objects
// are released. The next call to Session reconnects transparently.
// Returns the names that were closed.
func (m *Manager) ReapIdle(maxIdle time.Duration) []string {
	now := time.Now()
	var closed []string
	m.mu.Lock()
	for name, s := range m.sessions {
		if s.idleFor(now) > maxIdle {
			s.Close()
			delete(m.sessions, name)
			closed = append(closed, name)
		}
	}
	m.mu.Unlock()
	if len(closed) > 0 {
		m.bus.Emit(TopicContextsChanged, m.Contexts())
	}
	return closed
}

// Disconnect closes a session, cancelling every watch/log/exec stream under it.
func (m *Manager) Disconnect(name string) {
	m.mu.Lock()
	if s := m.sessions[name]; s != nil {
		s.Close()
		delete(m.sessions, name)
	}
	m.mu.Unlock()
}

func (m *Manager) Close() {
	if m.watcher != nil {
		_ = m.watcher.Close()
	}
	m.mu.Lock()
	for name, s := range m.sessions {
		s.Close()
		delete(m.sessions, name)
	}
	m.mu.Unlock()
}

// watchFiles hot-reloads when kubeconfig files change (e.g. `aws eks
// update-kubeconfig`). Parent dirs are watched because editors and CLIs
// replace files atomically via rename, which drops a file-level watch.
func (m *Manager) watchFiles() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	m.watcher = w
	for _, p := range m.precedence() {
		_ = w.Add(filepath.Dir(p))
	}
	go func() {
		var debounce *time.Timer
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if !m.isKubeconfig(ev.Name) {
					continue
				}
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(300*time.Millisecond, func() { _ = m.Reload() })
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
}

func (m *Manager) isKubeconfig(name string) bool {
	return slices.Contains(m.precedence(), name)
}
