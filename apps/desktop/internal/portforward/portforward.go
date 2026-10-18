// Package portforward forwards local ports to pods and services, like
// `kubectl port-forward`. Listeners bind to 127.0.0.1 only (never the LAN),
// and every forward is stopped when Leme quits.
package portforward

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"leme/internal/cluster"
	"leme/internal/platform/events"
)

const TopicChanged = "portforwards:changed"

type Request struct {
	Namespace  string `json:"namespace"`
	Kind       string `json:"kind"` // "pod" | "service"
	Name       string `json:"name"`
	RemotePort int32  `json:"remotePort"` // container port (pod) or service port (service)
	LocalPort  int32  `json:"localPort"`  // 0 = pick a free port
}

type Forward struct {
	ID         string `json:"id"`
	Cluster    string `json:"cluster"`
	Namespace  string `json:"namespace"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Pod        string `json:"pod"` // the pod actually connected (a service resolves to one)
	RemotePort int32  `json:"remotePort"`
	PodPort    int32  `json:"podPort"`
	LocalPort  int32  `json:"localPort"`
	State      string `json:"state"` // active | error
	Error      string `json:"error"`
	Started    string `json:"started"`
}

type entry struct {
	Forward
	cancel  context.CancelFunc
	release func()
}

type Manager struct {
	bus events.Emitter
	mu  sync.Mutex
	all map[string]*entry
}

func NewManager(bus events.Emitter) *Manager {
	return &Manager{bus: bus, all: map[string]*entry{}}
}

func (m *Manager) List() []Forward {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Forward, 0, len(m.all))
	for _, e := range m.all {
		out = append(out, e.Forward)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started < out[j].Started })
	return out
}

func (m *Manager) changed() { m.bus.Emit(TopicChanged, m.List()) }

// Start resolves the target, opens the local listener and returns once it is
// accepting connections (or failed).
func (m *Manager) Start(s *cluster.Session, req Request) (Forward, error) {
	ctx, cancel := context.WithTimeout(s.Ctx(), 20*time.Second)
	defer cancel()

	pod, podPort, err := resolve(ctx, s.Kube, req)
	if err != nil {
		return Forward{}, err
	}

	fctx, stop := context.WithCancel(s.Ctx()) // dies with the cluster session
	ready := make(chan struct{})
	fw, err := newForwarder(fctx, s, req.Namespace, pod, req.LocalPort, podPort, ready)
	if err != nil {
		stop()
		return Forward{}, err
	}
	errCh := make(chan error, 1)
	go func() { errCh <- fw.ForwardPorts() }()

	select {
	case <-ready:
	case err := <-errCh:
		stop()
		return Forward{}, friendly(err, req.LocalPort)
	case <-ctx.Done():
		stop()
		return Forward{}, fmt.Errorf("o port-forward não ficou pronto a tempo")
	}
	ports, err := fw.GetPorts()
	if err != nil || len(ports) == 0 {
		stop()
		return Forward{}, fmt.Errorf("não foi possível abrir a porta local: %v", err)
	}

	s.Acquire() // an active forward keeps the cluster session alive
	e := &entry{
		Forward: Forward{
			ID: newID(), Cluster: s.Name, Namespace: req.Namespace, Kind: req.Kind, Name: req.Name,
			Pod: pod, RemotePort: req.RemotePort, PodPort: podPort, LocalPort: int32(ports[0].Local),
			State: "active", Started: time.Now().UTC().Format(time.RFC3339),
		},
		cancel: stop, release: s.Release,
	}
	m.mu.Lock()
	m.all[e.ID] = e
	m.mu.Unlock()
	m.changed()

	// When the pod goes away (rollout, crash) the forward ends: keep it in the
	// list as an error so the user sees why it stopped and can restart it.
	go func() {
		err := <-errCh
		m.mu.Lock()
		cur, ok := m.all[e.ID]
		if ok && cur.State == "active" {
			cur.State = "error"
			cur.Error = "conexão encerrada"
			if err != nil {
				cur.Error = err.Error()
			}
		}
		m.mu.Unlock()
		e.cancel()
		e.release()
		if ok {
			m.changed()
		}
	}()
	return e.Forward, nil
}

// Stop ends a forward and removes it from the list.
func (m *Manager) Stop(id string) {
	m.mu.Lock()
	e, ok := m.all[id]
	if ok {
		delete(m.all, id)
	}
	m.mu.Unlock()
	if ok {
		e.cancel()
		m.changed()
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	all := m.all
	m.all = map[string]*entry{}
	m.mu.Unlock()
	for _, e := range all {
		e.cancel()
	}
}

func newForwarder(ctx context.Context, s *cluster.Session, ns, pod string, local, remote int32, ready chan struct{}) (*portforward.PortForwarder, error) {
	req := s.Kube.CoreV1().RESTClient().Post().Resource("pods").Namespace(ns).Name(pod).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(s.Config)
	if err != nil {
		return nil, err
	}
	spdyDialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())
	// WebSocket tunneling (K8s >= 1.31) with fallback to SPDY, like kubectl.
	wsDialer, err := portforward.NewSPDYOverWebsocketDialer(req.URL(), s.Config)
	if err != nil {
		return nil, err
	}
	dialer := portforward.NewFallbackDialer(wsDialer, spdyDialer, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	ports := []string{fmt.Sprintf("%d:%d", local, remote)}
	// 127.0.0.1 only: the forwarded service is never exposed to the network.
	return portforward.NewOnAddressesWithContext(ctx, dialer, []string{"127.0.0.1"}, ports, ready, io.Discard, io.Discard)
}

// resolve returns the pod and the pod port to connect to.
func resolve(ctx context.Context, kube kubernetes.Interface, req Request) (string, int32, error) {
	switch req.Kind {
	case "pod":
		p, err := kube.CoreV1().Pods(req.Namespace).Get(ctx, req.Name, metav1.GetOptions{})
		if err != nil {
			return "", 0, err
		}
		if p.Status.Phase != corev1.PodRunning {
			return "", 0, fmt.Errorf("o pod %s não está rodando (%s)", p.Name, p.Status.Phase)
		}
		return p.Name, req.RemotePort, nil
	case "service":
		svc, err := kube.CoreV1().Services(req.Namespace).Get(ctx, req.Name, metav1.GetOptions{})
		if err != nil {
			return "", 0, err
		}
		if len(svc.Spec.Selector) == 0 {
			return "", 0, fmt.Errorf("o service %s não tem selector (ExternalName ou endpoints manuais): encaminhe direto para um pod", svc.Name)
		}
		pods, err := kube.CoreV1().Pods(req.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(svc.Spec.Selector).String(),
		})
		if err != nil {
			return "", 0, err
		}
		return ServiceTarget(svc, pods.Items, req.RemotePort)
	}
	return "", 0, fmt.Errorf("tipo não suportado: %s", req.Kind)
}

// ServiceTarget picks a ready pod behind the service and translates the
// service port to the pod port (numeric or named targetPort), like kubectl.
func ServiceTarget(svc *corev1.Service, pods []corev1.Pod, servicePort int32) (string, int32, error) {
	var sp *corev1.ServicePort
	for i := range svc.Spec.Ports {
		if svc.Spec.Ports[i].Port == servicePort {
			sp = &svc.Spec.Ports[i]
		}
	}
	if sp == nil {
		return "", 0, fmt.Errorf("o service %s não expõe a porta %d", svc.Name, servicePort)
	}
	for _, p := range pods {
		if p.Status.Phase != corev1.PodRunning || p.DeletionTimestamp != nil || !podReady(p) {
			continue
		}
		if sp.TargetPort.IntVal == 0 && sp.TargetPort.StrVal == "" {
			return p.Name, sp.Port, nil // targetPort defaults to port
		}
		if sp.TargetPort.StrVal == "" {
			return p.Name, sp.TargetPort.IntVal, nil
		}
		for _, c := range p.Spec.Containers { // named targetPort
			for _, cp := range c.Ports {
				if cp.Name == sp.TargetPort.StrVal {
					return p.Name, cp.ContainerPort, nil
				}
			}
		}
	}
	return "", 0, fmt.Errorf("nenhum pod pronto atrás do service %s", svc.Name)
}

func podReady(p corev1.Pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func friendly(err error, local int32) error {
	if err == nil {
		return fmt.Errorf("o port-forward terminou antes de ficar pronto")
	}
	if strings.Contains(err.Error(), "unable to listen") || strings.Contains(err.Error(), "address already in use") {
		if local > 0 {
			return fmt.Errorf("a porta local %d já está em uso: escolha outra ou deixe automática", local)
		}
		return fmt.Errorf("não foi possível abrir uma porta local: %w", err)
	}
	if strings.Contains(err.Error(), "forbidden") {
		return fmt.Errorf("sem permissão para port-forward neste namespace: %w", err)
	}
	return err
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
