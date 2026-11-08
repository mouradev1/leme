package rancher

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"leme/internal/platform/secrets"
	"leme/internal/platform/settings"
)

// fakeRancher mimics the endpoints Leme uses.
type fakeRancher struct {
	mu        sync.Mutex
	token     string // the only valid token
	pending   map[string]*rsa.PublicKey
	ready     map[string]string // requestId -> encrypted token
	revoked   []string
	deletedAT []string
}

func (f *fakeRancher) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	auth := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer "+f.token }
	mux.HandleFunc("GET /rancherversion", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Version":"v2.9.3","GitCommit":"x"}`))
	})
	mux.HandleFunc("GET /v3-public/authProviders", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"local","type":"localProvider"},{"id":"keycloakoidc","type":"keyCloakOIDCProvider"}]}`))
	})
	mux.HandleFunc("GET /v3/projects", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) || r.URL.Query().Get("clusterId") != "c-b" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"c-b:p-1","name":"Pagamentos"},{"id":"c-b:p-2","name":"Default"}]}`))
	})
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("pong")) })
	mux.HandleFunc("GET /v3/clusters", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"c-b","name":"web","state":"active"},{"id":"local","name":"local","state":"active"}]}`))
	})
	mux.HandleFunc("GET /v3/users", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"name":"Moura","username":"moura"}]}`))
	})
	mux.HandleFunc("DELETE /v3/tokens/{name}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.revoked = append(f.revoked, r.PathValue("name"))
		f.mu.Unlock()
	})
	mux.HandleFunc("GET /v3-public/authTokens/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		enc, ok := f.ready[r.PathValue("id")]
		if !ok {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": enc, "expiresAt": "2030-01-01T00:00:00Z"})
	})
	mux.HandleFunc("DELETE /v3-public/authTokens/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.deletedAT = append(f.deletedAT, r.PathValue("id"))
		delete(f.ready, r.PathValue("id"))
		f.mu.Unlock()
	})
	return mux
}

// browserLogin plays the user: opens the URL, "logs in", and Rancher stores
// the token encrypted with the public key from the URL.
func (f *fakeRancher) browserLogin(t *testing.T, loginURL string) {
	u, err := url.Parse(loginURL)
	if err != nil || u.Path != "/dashboard/auth/login" || u.Query().Get("responseType") != "kubeconfig" {
		t.Errorf("unexpected login url %q", loginURL)
		return
	}
	raw, _ := base64.StdEncoding.DecodeString(u.Query().Get("publicKey"))
	var pub rsa.PublicKey
	if err := json.Unmarshal(raw, &pub); err != nil {
		t.Errorf("public key: %v", err)
		return
	}
	enc, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &pub, []byte(f.token), nil)
	if err != nil {
		t.Error(err)
		return
	}
	go func() {
		time.Sleep(30 * time.Millisecond) // the user takes a moment
		f.mu.Lock()
		f.ready[u.Query().Get("requestId")] = base64.StdEncoding.EncodeToString(enc)
		f.mu.Unlock()
	}()
}

type overlays struct {
	mu sync.Mutex
	m  map[string]*clientcmdapi.Config
}

func (o *overlays) SetOverlay(k string, c *clientcmdapi.Config) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if c == nil {
		delete(o.m, k)
	} else {
		o.m[k] = c
	}
}

type nopBus struct{}

func (nopBus) Emit(string, any) {}

func setup(t *testing.T) (*Manager, *fakeRancher, *overlays, secrets.Memory, string) {
	f := &fakeRancher{token: "token-abc:s3cret", pending: map[string]*rsa.PublicKey{}, ready: map[string]string{}}
	srv := httptest.NewTLSServer(f.handler(t))
	t.Cleanup(srv.Close)
	ov := &overlays{m: map[string]*clientcmdapi.Config{}}
	store := secrets.Memory{}
	var st settings.Settings
	m := NewManager(nopBus{}, ov, store)
	m.load = func() settings.Settings {
		b, _ := json.Marshal(st)
		var c settings.Settings
		_ = json.Unmarshal(b, &c)
		return c
	}
	m.save = func(s settings.Settings) error { st = s; return nil }
	m.client = func(base string) *Client { return &Client{Base: base, HTTP: srv.Client()} }
	return m, f, ov, store, srv.URL
}

func TestBrowserLoginListsClustersAndStoresToken(t *testing.T) {
	m, f, ov, store, base := setup(t)
	ctx := context.Background()
	st, err := m.Add(ctx, base+"/dashboard/home", "Prod")
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "logged-out" || st.Name != "prod" {
		t.Fatalf("after add: %+v", st)
	}

	st, err = m.Login(ctx, st.URL, func(u string) { f.browserLogin(t, u) })
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "ok" || st.User != "Moura" || strings.Join(st.Clusters, ",") != "prod/local,prod/web" {
		t.Fatalf("after login: %+v", st)
	}
	if store[st.URL] != f.token {
		t.Fatal("token not stored in the secret store")
	}
	if len(f.deletedAT) != 1 {
		t.Fatal("the one-time auth token must be deleted from the server")
	}
	cfg := ov.m[Origin+st.URL]
	c := cfg.Contexts["prod/web"]
	if c == nil || cfg.Clusters[c.Cluster].Server != st.URL+"/k8s/clusters/c-b" || cfg.AuthInfos[c.AuthInfo].Token != f.token {
		t.Fatalf("overlay: %+v", cfg)
	}
	if u, name, ok := m.Owner("prod/web"); !ok || u != st.URL || name != "prod" {
		t.Fatal("owner lookup")
	}

	// Token expires: status says so; logging in again brings it back.
	m.MarkExpired(st.URL)
	if m.List()[0].State != "expired" {
		t.Fatal("expected expired")
	}
	f.token = "token-def:n3w"
	if _, err := m.Login(ctx, st.URL, func(u string) { f.browserLogin(t, u) }); err != nil {
		t.Fatal(err)
	}
	if m.List()[0].State != "ok" || ov.m[Origin+st.URL].AuthInfos[Origin+st.URL].Token != "token-def:n3w" {
		t.Fatal("new token not applied")
	}

	// Logout revokes on the server and removes it locally; clusters stay listed.
	if err := m.Logout(ctx, st.URL); err != nil {
		t.Fatal(err)
	}
	if _, ok := store[st.URL]; ok || len(f.revoked) != 1 || f.revoked[0] != "token-def" {
		t.Fatalf("logout: store=%v revoked=%v", store, f.revoked)
	}
	if m.List()[0].State != "logged-out" || ov.m[Origin+st.URL].Contexts["prod/web"] == nil {
		t.Fatal("after logout")
	}

	if err := m.Remove(ctx, st.URL); err != nil {
		t.Fatal(err)
	}
	if len(m.List()) != 0 || ov.m[Origin+st.URL] != nil {
		t.Fatal("remove")
	}
}

func TestLoginWithKeyAndRefresh(t *testing.T) {
	m, f, _, _, base := setup(t)
	ctx := context.Background()
	st, _ := m.Add(ctx, base, "")
	if _, err := m.LoginWithKey(ctx, st.URL, "nonsense"); err == nil {
		t.Fatal("bad key format accepted")
	}
	if _, err := m.LoginWithKey(ctx, st.URL, "token-x:wrong"); err == nil {
		t.Fatal("rejected token accepted")
	}
	if _, err := m.LoginWithKey(ctx, st.URL, "Bearer "+f.token); err != nil {
		t.Fatal(err)
	}
	f.token = "token-other:x" // revoked elsewhere
	if _, err := m.Refresh(ctx, st.URL); err == nil || m.List()[0].State != "expired" {
		t.Fatal("refresh with a dead token must mark the server expired")
	}
}

func TestLoginCancel(t *testing.T) {
	m, _, _, _, base := setup(t)
	st, _ := m.Add(context.Background(), base, "x")
	done := make(chan error)
	go func() {
		_, err := m.Login(context.Background(), st.URL, func(string) {}) // user never logs in
		done <- err
	}()
	for i := 0; m.List()[0].State != "logging-in"; i++ { // key generation takes a moment
		if i > 200 {
			t.Fatal("expected logging-in")
		}
		time.Sleep(10 * time.Millisecond)
	}
	m.CancelLogin(st.URL)
	if err := <-done; err == nil {
		t.Fatal("expected cancel error")
	}
}

func TestNormalizeURL(t *testing.T) {
	ok := map[string]string{
		"rancher.acme.com":                      "https://rancher.acme.com",
		"https://Rancher.acme.com/dashboard/c/": "https://rancher.acme.com",
		"https://r.acme.com:8443":               "https://r.acme.com:8443",
	}
	for in, want := range ok {
		if got, err := NormalizeURL(in); err != nil || got != want {
			t.Errorf("%q -> %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "http://r.acme.com", "https://user:pw@r.acme.com"} {
		if _, err := NormalizeURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if shortName("https://rancher.prod.acme.com") != "rancher-prod" || shortName("https://r.acme.com") != "r" {
		t.Error("shortName")
	}
}

func TestServerInfo(t *testing.T) {
	m, _, _, _, base := setup(t)
	st, _ := m.Add(context.Background(), base, "x")
	info, err := m.Info(context.Background(), st.URL)
	if err != nil || info.Version != "v2.9.3" || strings.Join(info.Providers, ",") != "localProvider,keyCloakOIDCProvider" {
		t.Fatalf("%+v %v", info, err)
	}
}

func TestProjectNames(t *testing.T) {
	m, f, _, _, base := setup(t)
	ctx := context.Background()
	st, _ := m.Add(ctx, base, "prod")
	if _, err := m.LoginWithKey(ctx, st.URL, f.token); err != nil {
		t.Fatal(err)
	}
	got, err := m.ProjectNames(ctx, "prod/web")
	if err != nil || got["p-1"] != "Pagamentos" || got["p-2"] != "Default" {
		t.Fatalf("%v %v", got, err)
	}
	if got, _ := m.ProjectNames(ctx, "minikube"); len(got) != 0 {
		t.Fatal("non-Rancher context must have no projects")
	}
}
