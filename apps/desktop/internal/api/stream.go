package api

import (
	"context"
	"fmt"
	"time"

	"leme/internal/metrics"
	"leme/internal/resources"
	"leme/internal/stream"
)

// StreamAPI starts subscriptions. Protocol for every stream:
//  1. the UI subscribes to EventsOn("<kind>:<subID>") first,
//  2. then calls Watch*/StreamLogs/WatchMetrics with that subID,
//  3. on unmount it calls Unsubscribe(subID) and drops the listener.
type StreamAPI struct{ d *Deps }

// WatchResources streams any kind (built-in or CRD) as server-rendered table
// rows on "watch:<subID>". Cluster events are just ref={"",v1,events} and
// per-object events add fieldSelector=involvedObject.uid=<uid>.
func (a *StreamAPI) WatchResources(subID, cluster string, ref resources.Ref, q resources.Query) error {
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	rc := s.Kube.CoreV1().RESTClient()
	topic := "watch:" + subID
	a.d.stream(s, subID, func(ctx context.Context) {
		stream.WatchTable(ctx, rc, ref, q, func(ev stream.ResourceEvent) { a.d.Bus.Emit(topic, ev) })
	})
	return nil
}

func (a *StreamAPI) StreamLogs(subID, cluster string, opts stream.LogOptions) error {
	if len(opts.Targets) == 0 {
		return fmt.Errorf("no log targets")
	}
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	topic := "logs:" + subID
	a.d.stream(s, subID, func(ctx context.Context) {
		stream.StreamLogs(ctx, s.Kube, opts, func(ev stream.LogEvent) { a.d.Bus.Emit(topic, ev) })
	})
	return nil
}

type MetricsQuery struct {
	Scope           string `json:"scope"` // pods | nodes
	Namespace       string `json:"namespace"`
	IntervalSeconds int    `json:"intervalSeconds"`
	// Optional Prometheus source; empty Service = metrics-server.
	PromNamespace string `json:"promNamespace"`
	PromService   string `json:"promService"`
	PromPort      string `json:"promPort"`
}

func (a *StreamAPI) WatchMetrics(subID, cluster string, q MetricsQuery) error {
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	var p metrics.Provider = metrics.MetricsServer{C: s.Metrics}
	if q.PromService != "" {
		p = metrics.Prometheus{Kube: s.Kube, Namespace: q.PromNamespace, Service: q.PromService, Port: q.PromPort}
	}
	fetch := func(ctx context.Context) ([]metrics.Usage, error) { return p.Pods(ctx, q.Namespace) }
	if q.Scope == "nodes" {
		fetch = p.Nodes
	}
	interval := time.Duration(max(q.IntervalSeconds, 5)) * time.Second
	topic := "metrics:" + subID
	a.d.stream(s, subID, func(ctx context.Context) {
		metrics.Poll(ctx, interval, fetch, func(snap metrics.Snapshot) { a.d.Bus.Emit(topic, snap) })
	})
	return nil
}

func (a *StreamAPI) Unsubscribe(subID string) { a.d.Hub.Stop(subID) }

// ActiveSubscriptions is handy for a debug/status bar (leak detection).
func (a *StreamAPI) ActiveSubscriptions() int { return a.d.Hub.Count() }

// WatchPodMetrics polls per-container usage of one pod while its details
// panel is open ("metrics:<subID>"). The UI keeps the history for the chart.
func (a *StreamAPI) WatchPodMetrics(subID, cluster, namespace, pod string, intervalSeconds int) error {
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	ms := metrics.MetricsServer{C: s.Metrics}
	interval := time.Duration(max(intervalSeconds, 5)) * time.Second
	topic := "metrics:" + subID
	a.d.stream(s, subID, func(ctx context.Context) {
		metrics.Poll(ctx, interval, func(ctx context.Context) ([]metrics.Usage, error) {
			return ms.Containers(ctx, namespace, pod)
		}, func(snap metrics.Snapshot) { a.d.Bus.Emit(topic, snap) })
	})
	return nil
}
