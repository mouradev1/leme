package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type ContextInfo struct {
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	User      string `json:"user"`
	Namespace string `json:"namespace"`
	Server    string `json:"server"`
	Auth      string `json:"auth"`   // exec:aws | auth-provider:oidc | mtls | token | basic | none
	Source    string `json:"source"` // kubeconfig file the context came from
	Current   bool   `json:"current"`
	Connected bool   `json:"connected"`
}

func authKind(a *clientcmdapi.AuthInfo) string {
	switch {
	case a.Exec != nil:
		return "exec:" + filepath.Base(a.Exec.Command)
	case a.AuthProvider != nil:
		return "auth-provider:" + a.AuthProvider.Name
	case len(a.ClientCertificateData) > 0 || a.ClientCertificate != "":
		return "mtls"
	case a.Token != "" || a.TokenFile != "":
		return "token"
	case a.Username != "":
		return "basic"
	}
	return "none"
}

// fingerprint hashes the context + cluster + user stanzas so Reload can tell
// which sessions are affected by a kubeconfig edit.
func fingerprint(raw *clientcmdapi.Config, name string) string {
	c := raw.Contexts[name]
	if c == nil {
		return ""
	}
	b, _ := json.Marshal([]any{c, raw.Clusters[c.Cluster], raw.AuthInfos[c.AuthInfo]})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ScopedKubeconfig writes a minified, flattened kubeconfig containing only one
// context. The local terminal gets KUBECONFIG pointing at it, so commands typed
// there always act on the cluster the tab belongs to, never on whatever is
// "current" globally.
//
// The file holds credentials (flattened certs/tokens), so it lives in a
// private per-user dir (0700, file 0600) instead of the shared temp dir, is
// deleted when the shell exits, and leftovers from a crash are swept by
// CleanupScopedKubeconfigs at startup and shutdown.
func (m *Manager) ScopedKubeconfig(name string) (string, error) {
	m.mu.RLock()
	cfg := m.raw.DeepCopy()
	m.mu.RUnlock()
	if _, ok := cfg.Contexts[name]; !ok {
		return "", fmt.Errorf("context %q not found", name)
	}
	cfg.CurrentContext = name
	if err := clientcmdapi.MinifyConfig(cfg); err != nil {
		return "", err
	}
	if err := clientcmdapi.FlattenConfig(cfg); err != nil {
		return "", err
	}
	dir, err := scopedDir()
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, scopedPrefix+"*.yaml") // created 0600
	if err != nil {
		return "", err
	}
	_ = f.Close()
	return f.Name(), clientcmd.WriteToFile(*cfg, f.Name())
}

const scopedPrefix = "leme-kubeconfig-"

func scopedDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "leme", "kubeconfigs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, os.Chmod(dir, 0o700) // tighten if it pre-existed with looser perms
}

// CleanupScopedKubeconfigs deletes credential files left behind (crash,
// force-quit). Also sweeps the shared temp dir used by older builds.
func CleanupScopedKubeconfigs() {
	dirs := []string{os.TempDir()}
	if base, err := os.UserCacheDir(); err == nil { // pre-rename location
		legacy := filepath.Join(base, "btu", "kubeconfigs")
		dirs = append(dirs, legacy)
		defer os.Remove(legacy) // only succeeds once it's empty
	}
	if d, err := scopedDir(); err == nil {
		dirs = append(dirs, d)
	}
	for _, d := range dirs {
		matches, _ := filepath.Glob(filepath.Join(d, scopedPrefix+"*.yaml"))
		legacy, _ := filepath.Glob(filepath.Join(d, "btu-kubeconfig-*.yaml"))
		matches = append(matches, legacy...)
		for _, f := range matches {
			_ = os.Remove(f)
		}
	}
}
