// Package metrics provides CPU/memory usage from metrics-server or Prometheus
// behind one interface. Neither is watchable, so the UI subscribes and we poll
// only while a view is open.
package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Usage struct {
	Namespace   string `json:"namespace,omitempty"`
	Name        string `json:"name"`
	CPUMilli    int64  `json:"cpu"`    // millicores
	MemoryBytes int64  `json:"memory"` // bytes (working set)
}

type Snapshot struct {
	At    time.Time `json:"at"`
	Items []Usage   `json:"items"`
	Error string    `json:"error,omitempty"`
}

type Provider interface {
	Pods(ctx context.Context, namespace string) ([]Usage, error)
	Nodes(ctx context.Context) ([]Usage, error)
}

// ---- metrics-server (metrics.k8s.io) ----

type MetricsServer struct{ C metricsclient.Interface }

func (m MetricsServer) Pods(ctx context.Context, ns string) ([]Usage, error) {
	l, err := m.C.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Usage, 0, len(l.Items))
	for _, p := range l.Items {
		u := Usage{Namespace: p.Namespace, Name: p.Name}
		for _, c := range p.Containers {
			u.CPUMilli += c.Usage.Cpu().MilliValue()
			u.MemoryBytes += c.Usage.Memory().Value()
		}
		out = append(out, u)
	}
	return out, nil
}

func (m MetricsServer) Nodes(ctx context.Context) ([]Usage, error) {
	l, err := m.C.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Usage, 0, len(l.Items))
	for _, n := range l.Items {
		out = append(out, Usage{Name: n.Name, CPUMilli: n.Usage.Cpu().MilliValue(), MemoryBytes: n.Usage.Memory().Value()})
	}
	return out, nil
}

// ---- Prometheus through the API server's service proxy ----
// No port-forward and no extra network path: it reuses the cluster
// credentials (a configurable "Prometheus service address").

type Prometheus struct {
	Kube      kubernetes.Interface
	Namespace string // e.g. "monitoring"
	Service   string // e.g. "prometheus-operated"
	Port      string // e.g. "9090" or "http"
}

func (p Prometheus) Pods(ctx context.Context, ns string) ([]Usage, error) {
	sel := `container!="",pod!=""`
	if ns != "" {
		sel += fmt.Sprintf(`,namespace=%q`, ns)
	}
	return p.join(ctx, []string{"namespace", "pod"},
		fmt.Sprintf(`sum by (namespace,pod) (rate(container_cpu_usage_seconds_total{%s}[2m]))`, sel),
		fmt.Sprintf(`sum by (namespace,pod) (container_memory_working_set_bytes{%s})`, sel))
}

func (p Prometheus) Nodes(ctx context.Context) ([]Usage, error) {
	return p.join(ctx, []string{"", "node"},
		`sum by (node) (rate(container_cpu_usage_seconds_total{id="/"}[2m]))`,
		`sum by (node) (container_memory_working_set_bytes{id="/"})`)
}

func (p Prometheus) join(ctx context.Context, keys []string, cpuQ, memQ string) ([]Usage, error) {
	cpu, err := p.query(ctx, cpuQ)
	if err != nil {
		return nil, err
	}
	mem, err := p.query(ctx, memQ)
	if err != nil {
		return nil, err
	}
	byKey := map[string]*Usage{}
	get := func(m map[string]string) *Usage {
		k := m[keys[0]] + "/" + m[keys[1]]
		if byKey[k] == nil {
			byKey[k] = &Usage{Namespace: m[keys[0]], Name: m[keys[1]]}
		}
		return byKey[k]
	}
	for _, s := range cpu {
		get(s.labels).CPUMilli = int64(s.value * 1000)
	}
	for _, s := range mem {
		get(s.labels).MemoryBytes = int64(s.value)
	}
	out := make([]Usage, 0, len(byKey))
	for _, u := range byKey {
		out = append(out, *u)
	}
	return out, nil
}

type sample struct {
	labels map[string]string
	value  float64
}

func (p Prometheus) query(ctx context.Context, q string) ([]sample, error) {
	raw, err := p.Kube.CoreV1().Services(p.Namespace).
		ProxyGet("http", p.Service, p.Port, "api/v1/query", map[string]string{"query": q}).
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	var r struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  [2]any            `json:"value"` // [unixTime, "stringValue"]
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	if r.Status != "success" {
		return nil, fmt.Errorf("prometheus: %s", r.Error)
	}
	out := make([]sample, 0, len(r.Data.Result))
	for _, res := range r.Data.Result {
		s, _ := res.Value[1].(string)
		v, err := strconv.ParseFloat(s, 64)
		if err == nil {
			out = append(out, sample{labels: res.Metric, value: v})
		}
	}
	return out, nil
}

// Poll emits a Snapshot immediately and then every interval until ctx ends.
func Poll(ctx context.Context, interval time.Duration, fetch func(context.Context) ([]Usage, error), emit func(Snapshot)) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		fctx, cancel := context.WithTimeout(ctx, interval)
		items, err := fetch(fctx)
		cancel()
		snap := Snapshot{At: time.Now(), Items: items}
		if err != nil {
			snap.Error = err.Error()
		}
		if ctx.Err() != nil {
			return
		}
		emit(snap)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Containers returns per-container usage of one pod (metrics-server).
func (m MetricsServer) Containers(ctx context.Context, ns, pod string) ([]Usage, error) {
	pm, err := m.C.MetricsV1beta1().PodMetricses(ns).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]Usage, 0, len(pm.Containers))
	for _, c := range pm.Containers {
		out = append(out, Usage{Name: c.Name, CPUMilli: c.Usage.Cpu().MilliValue(), MemoryBytes: c.Usage.Memory().Value()})
	}
	return out, nil
}
