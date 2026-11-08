package rancher

import (
	"context"
	"errors"
	"fmt"
	"leme/internal/platform/i18n"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"leme/internal/platform/events"
	"leme/internal/platform/secrets"
	"leme/internal/platform/settings"
)

// TopicChanged carries []Status whenever a server is added, logs in/out or
// refreshes its clusters.
const TopicChanged = "rancher:changed"

// Origin marks contexts that come from a Rancher (ContextInfo.Source).
const Origin = "rancher:"

// Overlayer is the part of cluster.Manager this package needs.
type Overlayer interface {
	SetOverlay(key string, cfg *clientcmdapi.Config)
}

type Status struct {
	URL       string   `json:"url"`
	Name      string   `json:"name"`
	User      string   `json:"user"`
	State     string   `json:"state"` // ok | expired | logged-out | logging-in
	ExpiresAt string   `json:"expiresAt"`
	Clusters  []string `json:"clusters"` // context names in Leme
}

type Manager struct {
	bus     events.Emitter
	ov      Overlayer
	secrets secrets.Store
	load    func() settings.Settings
	save    func(settings.Settings) error
	client  func(base string) *Client
	now     func() time.Time

	mu      sync.Mutex
	expired map[string]bool                // server answered 401
	logins  map[string]*context.CancelFunc // browser logins in progress
	owner   map[string]string              // context name -> server URL
}

func NewManager(bus events.Emitter, ov Overlayer, store secrets.Store) *Manager {
	return &Manager{
		bus: bus, ov: ov, secrets: store,
		load: settings.Load, save: settings.Save, client: NewClient, now: time.Now,
		expired: map[string]bool{}, logins: map[string]*context.CancelFunc{}, owner: map[string]string{},
	}
}

// Start publishes the saved servers' clusters (no network: last known list).
func (m *Manager) Start() {
	for _, s := range m.load().Ranchers {
		m.apply(s)
	}
}

func (m *Manager) servers() []settings.RancherServer { return m.load().Ranchers }

func (m *Manager) find(u string) (settings.RancherServer, bool) {
	for _, s := range m.servers() {
		if s.URL == u {
			return s, true
		}
	}
	return settings.RancherServer{}, false
}

// update saves a changed server (or appends it).
func (m *Manager) update(srv settings.RancherServer) error {
	st := m.load()
	i := slices.IndexFunc(st.Ranchers, func(s settings.RancherServer) bool { return s.URL == srv.URL })
	if i < 0 {
		st.Ranchers = append(st.Ranchers, srv)
	} else {
		st.Ranchers[i] = srv
	}
	return m.save(st)
}

var nameRe = regexp.MustCompile(`[^a-z0-9-]+`)

// shortName: "https://rancher.prod.acme.com" -> "rancher-prod" style label.
func shortName(u string) string {
	host := strings.TrimPrefix(u, "https://")
	host, _, _ = strings.Cut(host, ":")
	parts := strings.Split(host, ".")
	if len(parts) > 2 {
		parts = parts[:len(parts)-2] // drop the company domain
	}
	n := strings.Trim(nameRe.ReplaceAllString(strings.ToLower(strings.Join(parts, "-")), "-"), "-")
	if n == "" {
		n = "rancher"
	}
	return n
}

func cleanName(n string) string {
	n = strings.Trim(nameRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(n)), "-"), "-")
	if len(n) > 40 {
		n = n[:40]
	}
	return n
}

// Add validates the address (it must answer like a Rancher) and saves it.
// The user logs in afterwards.
func (m *Manager) Add(ctx context.Context, rawURL, name string) (Status, error) {
	u, err := NormalizeURL(rawURL)
	if err != nil {
		return Status{}, err
	}
	if s, ok := m.find(u); ok {
		return m.status(s), nil // already there
	}
	if err := m.client(u).Ping(ctx); err != nil {
		return Status{}, err
	}
	n := cleanName(name)
	if n == "" {
		n = shortName(u)
	}
	for _, s := range m.servers() { // names prefix context names: keep them unique
		if s.Name == n {
			n += "-2"
		}
	}
	srv := settings.RancherServer{URL: u, Name: n}
	if err := m.update(srv); err != nil {
		return Status{}, err
	}
	m.emit()
	return m.status(srv), nil
}

// Remove logs out (revoking the token) and forgets the server.
func (m *Manager) Remove(ctx context.Context, u string) error {
	_ = m.Logout(ctx, u)
	st := m.load()
	st.Ranchers = slices.DeleteFunc(st.Ranchers, func(s settings.RancherServer) bool { return s.URL == u })
	if err := m.save(st); err != nil {
		return err
	}
	m.ov.SetOverlay(Origin+u, nil)
	m.mu.Lock()
	for c, o := range m.owner {
		if o == u {
			delete(m.owner, c)
		}
	}
	delete(m.expired, u)
	m.mu.Unlock()
	m.emit()
	return nil
}

// Login opens the browser (via open) and waits for the user, up to 5 minutes.
func (m *Manager) Login(ctx context.Context, u string, open func(string)) (Status, error) {
	srv, ok := m.find(u)
	if !ok {
		return Status{}, fmt.Errorf(i18n.T("Rancher %s não cadastrado", "Rancher %s is not registered"), u)
	}
	c := m.client(u)
	l, err := c.StartLogin()
	if err != nil {
		return Status{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	m.mu.Lock()
	if prev := m.logins[u]; prev != nil {
		(*prev)() // a second click restarts the login
	}
	mine := &cancel
	m.logins[u] = mine
	m.mu.Unlock()
	m.emit()

	open(l.URL)
	tok, err := l.Wait(ctx, 2*time.Second)
	m.mu.Lock()
	if m.logins[u] == mine { // not replaced by a newer click
		delete(m.logins, u)
	}
	m.mu.Unlock()
	if err != nil {
		m.emit()
		if errors.Is(err, context.DeadlineExceeded) {
			return Status{}, errors.New(i18n.T(i18n.T("o login no navegador não foi concluído em 5 minutos", "the browser login was not completed within 5 minutes"), "the browser login was not completed within 5 minutes"))
		}
		if errors.Is(err, context.Canceled) {
			return Status{}, errors.New(i18n.T(i18n.T("login cancelado", "login cancelled"), "login cancelled"))
		}
		return Status{}, err
	}
	return m.useToken(ctx, srv, tok)
}

// Info returns the server's version and login methods (public endpoints).
func (m *Manager) Info(ctx context.Context, u string) (Info, error) {
	if _, ok := m.find(u); !ok {
		return Info{}, fmt.Errorf(i18n.T("Rancher %s não cadastrado", "Rancher %s is not registered"), u)
	}
	return m.client(u).ServerInfo(ctx), nil
}

// CancelLogin stops waiting for a browser login.
func (m *Manager) CancelLogin(u string) {
	m.mu.Lock()
	if c := m.logins[u]; c != nil {
		(*c)()
	}
	m.mu.Unlock()
}

// LoginWithKey uses an API key created in Rancher's UI ("token-xxxx:yyyy").
// Plan B when the browser login doesn't work with a server's version.
func (m *Manager) LoginWithKey(ctx context.Context, u, key string) (Status, error) {
	srv, ok := m.find(u)
	if !ok {
		return Status{}, fmt.Errorf(i18n.T("Rancher %s não cadastrado", "Rancher %s is not registered"), u)
	}
	key = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(key), "Bearer "))
	if !strings.Contains(key, ":") || strings.ContainsAny(key, " \n\t\"'\\") {
		return Status{}, errors.New(i18n.T(i18n.T("a chave deve ter o formato token-xxxxx:yyyy (o \"Bearer Token\" mostrado ao criar a API key)", "the key must look like token-xxxxx:yyyy (the \"Bearer Token\" shown when the API key is created)"), "the key must look like token-xxxxx:yyyy (the \"Bearer Token\" shown when the API key is created)"))
	}
	return m.useToken(ctx, srv, Token{Token: key})
}

// useToken checks the token, stores it, lists the clusters and publishes them.
func (m *Manager) useToken(ctx context.Context, srv settings.RancherServer, tok Token) (Status, error) {
	c := m.client(srv.URL)
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	list, err := c.Clusters(cctx, tok.Token)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return Status{}, errors.New(i18n.T(i18n.T("o Rancher recusou o token (expirado ou sem permissão)", "Rancher refused the token (expired or without permission)"), "Rancher refused the token (expired or without permission)"))
		}
		return Status{}, err
	}
	if err := m.secrets.Set(srv.URL, tok.Token); err != nil {
		return Status{}, fmt.Errorf(i18n.T("não foi possível guardar o token no cofre do sistema: %w", "could not store the token in the system keychain: %w"), err)
	}
	old, _ := m.find(srv.URL)
	srv = old
	srv.ExpiresAt = tok.ExpiresAt
	srv.User = c.Me(cctx, tok.Token)
	srv.Clusters = toSaved(list)
	if err := m.update(srv); err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	delete(m.expired, srv.URL)
	m.mu.Unlock()
	m.apply(srv)
	m.emit()
	return m.status(srv), nil
}

func toSaved(list []Cluster) []settings.RancherCluster {
	out := make([]settings.RancherCluster, 0, len(list))
	for _, c := range list {
		if c.ID == "" || c.Name == "" {
			continue
		}
		out = append(out, settings.RancherCluster{ID: c.ID, Name: c.Name})
	}
	slices.SortFunc(out, func(a, b settings.RancherCluster) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// Refresh lists the server's clusters again (new clusters show up).
func (m *Manager) Refresh(ctx context.Context, u string) (Status, error) {
	srv, ok := m.find(u)
	if !ok {
		return Status{}, fmt.Errorf(i18n.T("Rancher %s não cadastrado", "Rancher %s is not registered"), u)
	}
	tok, err := m.secrets.Get(u)
	if err != nil {
		return m.status(srv), fmt.Errorf(i18n.T("entre no Rancher %s primeiro", "log in to Rancher %s first"), srv.Name)
	}
	list, err := m.client(u).Clusters(ctx, tok)
	if errors.Is(err, ErrUnauthorized) {
		m.MarkExpired(u)
		return m.status(srv), fmt.Errorf(i18n.T("a sessão do Rancher %s expirou: entre de novo", "the Rancher %s session expired: log in again"), srv.Name)
	}
	if err != nil {
		return m.status(srv), err
	}
	srv.Clusters = toSaved(list)
	if err := m.update(srv); err != nil {
		return Status{}, err
	}
	m.apply(srv)
	m.emit()
	return m.status(srv), nil
}

// RefreshAll lists the clusters of every logged-in server again (at startup).
// Failures are quiet; an expired token marks the server as expired.
func (m *Manager) RefreshAll(ctx context.Context) {
	for _, s := range m.servers() {
		if !m.HasToken(s.URL) {
			continue
		}
		rctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		_, _ = m.Refresh(rctx, s.URL)
		cancel()
	}
}

// Logout revokes the token on the server and removes it from this computer.
// The clusters stay listed (as logged out) until the server is removed.
func (m *Manager) Logout(ctx context.Context, u string) error {
	tok, err := m.secrets.Get(u)
	if err == nil {
		lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_ = m.client(u).Logout(lctx, tok) // best effort: offline still logs out locally
		cancel()
	}
	if err := m.secrets.Delete(u); err != nil {
		return err
	}
	if srv, ok := m.find(u); ok {
		srv.ExpiresAt, srv.User = "", ""
		_ = m.update(srv)
		m.apply(srv)
	}
	m.emit()
	return nil
}

// MarkExpired is called when a cluster of this server answered 401.
func (m *Manager) MarkExpired(u string) {
	m.mu.Lock()
	changed := !m.expired[u]
	m.expired[u] = true
	m.mu.Unlock()
	if changed {
		m.emit()
	}
}

// Owner tells which Rancher a context comes from.
func (m *Manager) Owner(context string) (url, name string, ok bool) {
	m.mu.Lock()
	u, ok := m.owner[context]
	m.mu.Unlock()
	if !ok {
		return "", "", false
	}
	srv, _ := m.find(u)
	return u, srv.Name, true
}

// ProjectNames maps project id to name for the cluster behind a context
// (empty for contexts that don't come from a logged-in Rancher).
func (m *Manager) ProjectNames(ctx context.Context, context string) (map[string]string, error) {
	u, _, ok := m.Owner(context)
	if !ok {
		return map[string]string{}, nil
	}
	srv, _ := m.find(u)
	var id string
	for _, c := range srv.Clusters {
		if srv.Name+"/"+c.Name == context {
			id = c.ID
		}
	}
	tok, err := m.secrets.Get(u)
	if id == "" || err != nil {
		return map[string]string{}, nil
	}
	return m.client(u).Projects(ctx, tok, id)
}

// HasToken: false means the user must log in before connecting.
func (m *Manager) HasToken(u string) bool {
	_, err := m.secrets.Get(u)
	return err == nil
}

// apply publishes the server's clusters as contexts "<server>/<cluster>".
func (m *Manager) apply(srv settings.RancherServer) {
	tok, _ := m.secrets.Get(srv.URL) // "" when logged out: listed, connecting asks to log in
	c := m.client(srv.URL)
	cfg := clientcmdapi.NewConfig()
	user := Origin + srv.URL
	cfg.AuthInfos[user] = &clientcmdapi.AuthInfo{Token: tok, LocationOfOrigin: Origin + srv.URL}
	names := make([]string, 0, len(srv.Clusters))
	for _, cl := range srv.Clusters {
		ctxName := srv.Name + "/" + cl.Name
		key := Origin + srv.URL + "/" + cl.ID
		cfg.Clusters[key] = &clientcmdapi.Cluster{Server: c.ProxyURL(cl.ID), LocationOfOrigin: Origin + srv.URL}
		cfg.Contexts[ctxName] = &clientcmdapi.Context{Cluster: key, AuthInfo: user, LocationOfOrigin: Origin + srv.URL}
		names = append(names, ctxName)
	}
	m.mu.Lock()
	for ctxName, o := range m.owner {
		if o == srv.URL {
			delete(m.owner, ctxName)
		}
	}
	for _, n := range names {
		m.owner[n] = srv.URL
	}
	m.mu.Unlock()
	m.ov.SetOverlay(Origin+srv.URL, cfg)
}

func (m *Manager) status(srv settings.RancherServer) Status {
	st := Status{URL: srv.URL, Name: srv.Name, User: srv.User, ExpiresAt: srv.ExpiresAt, Clusters: []string{}}
	for _, c := range srv.Clusters {
		st.Clusters = append(st.Clusters, srv.Name+"/"+c.Name)
	}
	m.mu.Lock()
	logging, expired := m.logins[srv.URL] != nil, m.expired[srv.URL]
	m.mu.Unlock()
	switch {
	case logging:
		st.State = "logging-in"
	case !m.HasToken(srv.URL):
		st.State = "logged-out"
	case expired || pastExpiry(srv.ExpiresAt, m.now()):
		st.State = "expired"
	default:
		st.State = "ok"
	}
	return st
}

func pastExpiry(at string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, at)
	return err == nil && !t.IsZero() && now.After(t)
}

func (m *Manager) List() []Status {
	out := []Status{}
	for _, s := range m.servers() {
		out = append(out, m.status(s))
	}
	return out
}

func (m *Manager) emit() { m.bus.Emit(TopicChanged, m.List()) }
