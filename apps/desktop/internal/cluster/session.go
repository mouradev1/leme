package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Session is everything needed to talk to one cluster. All clients share a
// single *http.Client (one TLS pool / HTTP2 connection), and every stream
// started for this cluster derives from Ctx(), so Close() tears it all down.
type Session struct {
	Name         string
	Version      string
	Config       *rest.Config
	ClientConfig clientcmd.ClientConfig
	Kube         kubernetes.Interface
	Dynamic      dynamic.Interface
	Discovery    discovery.CachedDiscoveryInterface
	Mapper       meta.ResettableRESTMapper
	Metrics      metricsclient.Interface

	ctx         context.Context
	cancel      context.CancelFunc
	fingerprint string

	// Idle tracking: refs counts live streams/terminals; lastUsed is the last
	// API call. Sessions with no refs and no recent use are closed by the
	// Manager's reaper (see ReapIdle) to give memory and connections back.
	refs     atomic.Int64
	lastUsed atomic.Int64
}

func newSession(ctx context.Context, name string, cc clientcmd.ClientConfig) (*Session, error) {
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build client config for %q: %w", name, err)
	}
	// Defaults (5 QPS / 10 burst) throttle a UI that opens many views at once.
	cfg.QPS, cfg.Burst = 50, 100
	cfg.UserAgent = "leme"

	hc, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}
	kube, err := kubernetes.NewForConfigAndClient(cfg, hc)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfigAndClient(cfg, hc)
	if err != nil {
		return nil, err
	}
	mc, err := metricsclient.NewForConfigAndClient(cfg, hc)
	if err != nil {
		return nil, err
	}
	disc := memory.NewMemCacheClient(kube.Discovery())

	s := &Session{
		Name: name, Config: cfg, ClientConfig: cc,
		Kube: kube, Dynamic: dyn, Discovery: disc, Metrics: mc,
		Mapper: restmapper.NewDeferredDiscoveryRESTMapper(disc),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.Touch()

	// Probe /version: fails fast on bad certs, expired SSO, wrong endpoint.
	// Generous timeout because exec plugins may open a browser for SSO login.
	pctx, pcancel := context.WithTimeout(ctx, 30*time.Second)
	defer pcancel()
	raw, err := kube.CoreV1().RESTClient().Get().AbsPath("/version").Do(pctx).Raw()
	if err != nil {
		s.cancel()
		return nil, fmt.Errorf("cluster %q unreachable: %w", name, err)
	}
	var v version.Info
	if json.Unmarshal(raw, &v) == nil {
		s.Version = v.GitVersion
	}
	return s, nil
}

// Touch marks the session as used now.
func (s *Session) Touch() { s.lastUsed.Store(time.Now().UnixNano()) }

// Acquire/Release bracket long-lived users (watches, logs, exec shells) so an
// idle-looking session is never reaped while something is streaming from it.
func (s *Session) Acquire() { s.refs.Add(1); s.Touch() }
func (s *Session) Release() { s.refs.Add(-1); s.Touch() }

func (s *Session) idleFor(now time.Time) time.Duration {
	if s.refs.Load() > 0 {
		return 0
	}
	return now.Sub(time.Unix(0, s.lastUsed.Load()))
}

// Ctx is cancelled when the session closes; use it as parent for streams.
func (s *Session) Ctx() context.Context { return s.ctx }

func (s *Session) Close() { s.cancel() }

// DefaultNamespace is the namespace set on the kubeconfig context.
func (s *Session) DefaultNamespace() string {
	ns, _, err := s.ClientConfig.Namespace()
	if err != nil || ns == "" {
		return "default"
	}
	return ns
}
