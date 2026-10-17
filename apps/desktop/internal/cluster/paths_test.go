package cluster

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type nopBus struct{}

func (nopBus) Emit(string, any) {}

func writeKubeconfig(t *testing.T, path, ctx string) {
	t.Helper()
	data := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters: [{name: %[1]s, cluster: {server: "https://%[1]s.example:6443"}}]
users: [{name: %[1]s, user: {token: x}}]
contexts: [{name: %[1]s, context: {cluster: %[1]s, user: %[1]s}}]
current-context: %[1]s
`, ctx)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func names(m *Manager) []string {
	var out []string
	for _, c := range m.Contexts() {
		out = append(out, c.Name)
	}
	return out
}

func TestAddRemoveKubeconfig(t *testing.T) {
	dir := t.TempDir()
	def := filepath.Join(dir, "default")
	other := filepath.Join(dir, "somewhere", "else", "team.yaml")
	_ = os.MkdirAll(filepath.Dir(other), 0o700)
	writeKubeconfig(t, def, "local")
	writeKubeconfig(t, other, "team-prod")
	t.Setenv("KUBECONFIG", def) // never touch the developer's real kubeconfig

	m, err := NewManager(nopBus{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if got := names(m); len(got) != 1 || got[0] != "local" {
		t.Fatalf("initial contexts = %v", got)
	}

	n, err := m.AddPath(other)
	if err != nil || n != 1 {
		t.Fatalf("AddPath = %d, %v", n, err)
	}
	if got := names(m); len(got) != 2 || got[1] != "team-prod" {
		t.Fatalf("after add = %v", got)
	}
	if n, _ := m.AddPath(other); n != 1 || len(m.ExtraPaths()) != 1 {
		t.Fatalf("adding twice must not duplicate: %v", m.ExtraPaths())
	}
	for _, f := range m.Files() {
		if f.Path == other && (!f.Removable || f.Contexts != 1) {
			t.Fatalf("bad file info %+v", f)
		}
		if f.Path == def && f.Removable {
			t.Fatal("default kubeconfig must not be removable")
		}
	}

	if _, err := m.AddPath(dir); err == nil {
		t.Fatal("directory must be rejected")
	}
	bad := filepath.Join(dir, "bad")
	_ = os.WriteFile(bad, []byte("not: [valid"), 0o600)
	if _, err := m.AddPath(bad); err == nil {
		t.Fatal("invalid yaml must be rejected")
	}

	if err := m.RemovePath(other); err != nil {
		t.Fatal(err)
	}
	if got := names(m); len(got) != 1 {
		t.Fatalf("after remove = %v", got)
	}
	if err := m.RemovePath(def); err == nil {
		t.Fatal("removing the default kubeconfig must fail")
	}
}

func TestScopedKubeconfigIsPrivateAndCleaned(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, ".cache"))
	cfg := filepath.Join(dir, "config")
	writeKubeconfig(t, cfg, "local")
	t.Setenv("KUBECONFIG", cfg)

	m, err := NewManager(nopBus{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	p, err := m.ScopedKubeconfig("local")
	if err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("kubeconfig file perms = %v, want 0600", fi.Mode().Perm())
	}
	if di, _ := os.Stat(filepath.Dir(p)); di.Mode().Perm() != 0o700 {
		t.Fatalf("kubeconfig dir perms = %v, want 0700", di.Mode().Perm())
	}
	if filepath.Dir(p) == filepath.Clean(os.TempDir()) {
		t.Fatal("credentials must not be written to the shared temp dir")
	}
	CleanupScopedKubeconfigs()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("cleanup left the credentials file behind")
	}
}
