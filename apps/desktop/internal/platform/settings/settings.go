// Package settings persists user preferences in the OS config dir
// (~/Library/Application Support/leme on macOS, %AppData%\leme on Windows,
// ~/.config/leme on Linux).
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	KubeconfigPaths []string `json:"kubeconfigPaths"`
	// Update notice: on by default; the user can turn it off or skip a version.
	UpdateCheckDisabled bool   `json:"updateCheckDisabled,omitempty"`
	SkippedVersion      string `json:"skippedVersion,omitempty"`
	// Rancher servers. Tokens are NOT here: they live in the OS password store.
	Ranchers []RancherServer `json:"ranchers,omitempty"`
	// Per-cluster safety marks, by kubeconfig context name.
	Clusters map[string]ClusterPrefs `json:"clusters,omitempty"`
	// Node shell (privileged helper pod): can be turned off, or use an image
	// from an internal registry (pinned by digest).
	NodeShellDisabled bool   `json:"nodeShellDisabled,omitempty"`
	NodeShellImage    string `json:"nodeShellImage,omitempty"`
	// Commands that kubeconfigs may run to get credentials (exec plugins),
	// approved by the user once.
	ApprovedExec []string `json:"approvedExec,omitempty"`
}

// ClusterPrefs: Production asks for the cluster name before risky actions;
// ReadOnly blocks every change (enforced in the backend, not just hidden).
type ClusterPrefs struct {
	Production bool `json:"production,omitempty"`
	ReadOnly   bool `json:"readOnly,omitempty"`
}

// RancherServer is a Rancher the user added. Clusters is the last list seen,
// so they show up right away (and while logged out) before a refresh.
type RancherServer struct {
	URL       string           `json:"url"`
	Name      string           `json:"name"`
	User      string           `json:"user,omitempty"`
	ExpiresAt string           `json:"expiresAt,omitempty"`
	Clusters  []RancherCluster `json:"clusters,omitempty"`
}

type RancherCluster struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func file() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "leme", "settings.json"), nil
}

// Load never fails: a missing or corrupt file yields defaults.
func Load() Settings {
	var s Settings
	if p, err := file(); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &s)
		}
	}
	return s
}

// Save writes atomically (temp file + rename) so a crash never truncates it.
func Save(s Settings) error {
	p, err := file()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// legacyName is the app's pre-rename name; its data dirs are migrated once.
const legacyName = "btu"

// MigrateLegacy moves settings and pasted kubeconfigs from the pre-rename
// data dir (".../btu") to the current one (".../leme") on first start, and
// rewrites the saved kubeconfig paths that pointed inside it.
func MigrateLegacy() error {
	base, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	oldDir, newDir := filepath.Join(base, legacyName), filepath.Join(base, "leme")
	if _, err := os.Stat(oldDir); err != nil {
		return nil // nothing to migrate
	}
	if _, err := os.Stat(newDir); err == nil {
		return nil // already migrated (or both exist): never overwrite
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return err
	}
	s := Load()
	prefix := oldDir + string(filepath.Separator)
	for i, p := range s.KubeconfigPaths {
		if strings.HasPrefix(p, prefix) {
			s.KubeconfigPaths[i] = filepath.Join(newDir, strings.TrimPrefix(p, prefix))
		}
	}
	return Save(s)
}
