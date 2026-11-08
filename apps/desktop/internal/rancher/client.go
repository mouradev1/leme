// Package rancher connects Leme to Rancher servers: log in through the
// browser (whatever the server uses: Keycloak, AD, GitHub...), list the
// clusters the user may see, and reach them through Rancher's proxy
// (https://<rancher>/k8s/clusters/<id>) with one personal token. No kubeconfig
// files are written: when the token expires, logging in again fixes every
// cluster of that server at once.
package rancher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"leme/internal/platform/i18n"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrUnauthorized: the token is missing, expired or revoked.
var ErrUnauthorized = errors.New("rancher: not logged in or session expired")

type Cluster struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type Client struct {
	Base string // https://rancher.example.com (no trailing slash)
	HTTP *http.Client
}

func NewClient(base string) *Client {
	return &Client{Base: base, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// NormalizeURL accepts what users paste (with /dashboard/..., trailing slash,
// no scheme) and returns scheme://host[:port]. Only https is allowed: the
// token travels with every request.
func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New(i18n.T(i18n.T("informe o endereço do Rancher", "enter the Rancher address"), "enter the Rancher address"))
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf(i18n.T("endereço inválido: %q", "invalid address: %q"), raw)
	}
	if u.Scheme != "https" {
		return "", errors.New(i18n.T(i18n.T("use https:// (o token do Rancher não pode trafegar sem criptografia)", "use https:// (the Rancher token must not travel unencrypted)"), "use https:// (the Rancher token must not travel unencrypted)"))
	}
	if u.User != nil {
		return "", errors.New(i18n.T(i18n.T("o endereço não pode conter usuário ou senha", "the address must not contain a user or password"), "the address must not contain a user or password"))
	}
	return "https://" + strings.ToLower(u.Host), nil
}

func (c *Client) do(ctx context.Context, method, path, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "leme")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body := io.LimitReader(resp.Body, 8<<20)
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode >= 300:
		b, _ := io.ReadAll(io.LimitReader(body, 512))
		return fmt.Errorf("rancher: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, body)
		return nil
	}
	if s, ok := out.(*string); ok {
		b, err := io.ReadAll(body)
		*s = string(b)
		return err
	}
	return json.NewDecoder(body).Decode(out)
}

var errNotFound = errors.New("rancher: not found")

// Ping checks that the address is a Rancher server.
func (c *Client) Ping(ctx context.Context) error {
	var s string
	if err := c.do(ctx, http.MethodGet, "/ping", "", &s); err != nil {
		return fmt.Errorf(i18n.T("não foi possível falar com o Rancher em %s: %w", "could not reach Rancher at %s: %w"), c.Base, err)
	}
	if strings.TrimSpace(s) != "pong" {
		return fmt.Errorf(i18n.T("%s não parece ser um Rancher", "%s does not look like a Rancher server"), c.Base)
	}
	return nil
}

// Info is public data about a server, used to explain login problems.
type Info struct {
	Version   string   `json:"version"`
	Providers []string `json:"providers"` // enabled login methods, e.g. keyCloakOIDCProvider
}

// ServerInfo reads the version and the enabled login methods (both public,
// no login needed). Best effort: empty fields when the server hides them.
func (c *Client) ServerInfo(ctx context.Context) Info {
	var info Info
	var v struct {
		Version string `json:"Version"`
	}
	if c.do(ctx, http.MethodGet, "/rancherversion", "", &v) == nil {
		info.Version = v.Version
	}
	var p struct {
		Data []struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	if c.do(ctx, http.MethodGet, "/v3-public/authProviders", "", &p) == nil {
		for _, d := range p.Data {
			info.Providers = append(info.Providers, d.Type)
		}
	}
	return info
}

// Clusters lists the clusters the token's user may see.
func (c *Client) Clusters(ctx context.Context, token string) ([]Cluster, error) {
	var out struct {
		Data []Cluster `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/v3/clusters?limit=-1", token, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Projects maps project id ("p-abc") to its name for one cluster.
func (c *Client) Projects(ctx context.Context, token, clusterID string) (map[string]string, error) {
	var out struct {
		Data []struct {
			ID   string `json:"id"` // "c-xyz:p-abc"
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/v3/projects?limit=-1&clusterId="+url.QueryEscape(clusterID), token, &out); err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, p := range out.Data {
		_, id, _ := strings.Cut(p.ID, ":")
		if id == "" {
			id = p.ID
		}
		m[id] = p.Name
	}
	return m, nil
}

// Me returns the logged-in user's display name (best effort).
func (c *Client) Me(ctx context.Context, token string) string {
	var out struct {
		Data []struct {
			Name     string `json:"name"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if c.do(ctx, http.MethodGet, "/v3/users?me=true", token, &out) != nil || len(out.Data) == 0 {
		return ""
	}
	if out.Data[0].Name != "" {
		return out.Data[0].Name
	}
	return out.Data[0].Username
}

// Logout revokes the token on the server ("token-abc:secret" -> token-abc).
func (c *Client) Logout(ctx context.Context, token string) error {
	name, _, ok := strings.Cut(token, ":")
	if !ok || name == "" {
		return nil
	}
	err := c.do(ctx, http.MethodDelete, "/v3/tokens/"+url.PathEscape(name), token, nil)
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, errNotFound) {
		return nil // already expired or gone
	}
	return err
}

// ProxyURL is the API server address of a cluster, through Rancher.
func (c *Client) ProxyURL(clusterID string) string {
	return c.Base + "/k8s/clusters/" + url.PathEscape(clusterID)
}
