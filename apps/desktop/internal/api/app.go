// Package api is the only package bound to Wails. Each exported method on
// these structs becomes a typed TS function in frontend/wailsjs/go/api/*.
// Keep it thin: validate, resolve the cluster session, delegate.
//
// Every call takes the kubeconfig context name explicitly. The "active
// cluster" is purely a UI concept, which makes multi-cluster tabs and
// switching without restart fall out naturally.
package api

import (
	"context"
	"runtime/debug"
	"time"

	"leme/internal/cluster"
	"leme/internal/platform/events"
	"leme/internal/portforward"
	"leme/internal/stream"
	"leme/internal/terminal"
)

type Deps struct {
	Bus       *events.Bus
	Clusters  *cluster.Manager
	Hub       *stream.Hub
	Terminals *terminal.Manager
	Forwards  *portforward.Manager
	Version   string
}

type App struct {
	d        *Deps
	Cluster  *ClusterAPI
	Resource *ResourceAPI
	Stream   *StreamAPI
	Terminal *TerminalAPI
	Helm     *HelmAPI
	AppInfo  *AppAPI
	Forward  *PortForwardAPI
}

func New(d *Deps) *App {
	return &App{
		d:        d,
		Cluster:  &ClusterAPI{d},
		Resource: &ResourceAPI{d},
		Stream:   &StreamAPI{d},
		Terminal: &TerminalAPI{d},
		Helm:     &HelmAPI{d},
		AppInfo:  &AppAPI{d},
		Forward:  &PortForwardAPI{d},
	}
}

// IdleClusterTimeout: clusters with nothing open on screen for this long are
// disconnected (connections, caches and client objects freed). Reconnecting
// is automatic and instant on the next click.
const IdleClusterTimeout = 6 * time.Hour

func (a *App) Startup(ctx context.Context) {
	a.d.Bus.Bind(ctx)
	cluster.CleanupScopedKubeconfigs() // credentials left by a previous crash
	go a.reapIdle(ctx)
}

func (a *App) reapIdle(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if closed := a.d.Clusters.ReapIdle(IdleClusterTimeout); len(closed) > 0 {
				debug.FreeOSMemory() // hand the freed heap back to the OS right away
			}
		}
	}
}

func (a *App) Shutdown(context.Context) {
	a.d.Hub.StopAll()
	a.d.Terminals.CloseAll()
	a.d.Forwards.StopAll()
	a.d.Clusters.Close()
	cluster.CleanupScopedKubeconfigs() // shells may still be exiting; don't leave creds behind
}

func (a *App) Bindings() []any {
	return []any{a.Cluster, a.Resource, a.Stream, a.Terminal, a.Helm, a.AppInfo, a.Forward}
}

// session resolves (and lazily connects) a cluster with a bounded wait.
func (d *Deps) session(name string) (*cluster.Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	s, err := d.Clusters.Session(ctx, name)
	if err == nil {
		s.Touch()
	}
	return s, err
}

// stream runs a subscription that keeps the cluster session alive (not
// reapable as idle) for as long as it streams.
func (d *Deps) stream(s *cluster.Session, id string, fn func(ctx context.Context)) {
	d.Hub.Start(s.Ctx(), id, func(ctx context.Context) {
		s.Acquire()
		defer s.Release()
		fn(ctx)
	})
}

func reqCtx(s *cluster.Session) (context.Context, context.CancelFunc) {
	return context.WithTimeout(s.Ctx(), 30*time.Second)
}
