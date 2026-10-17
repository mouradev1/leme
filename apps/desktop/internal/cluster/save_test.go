package cluster

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const pasted = `apiVersion: v1
kind: Config
clusters: [{name: c1, cluster: {server: "https://c1.example:6443"}}]
users: [{name: u1, user: {token: secret-token}}]
contexts:
- {name: team-a, context: {cluster: c1, user: u1}}
- {name: local, context: {cluster: c1, user: u1}}
current-context: team-a
`

func TestSaveKubeconfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	def := filepath.Join(dir, "default")
	writeKubeconfig(t, def, "local")
	t.Setenv("KUBECONFIG", def)

	m, err := NewManager(nopBus{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	saved, err := m.SaveKubeconfig("", pasted)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(saved.Path) != "team-a.yaml" {
		t.Fatalf("file name = %s", saved.Path)
	}
	if !slices.Equal(saved.Contexts, []string{"local", "team-a"}) || !slices.Equal(saved.Shadowed, []string{"local"}) {
		t.Fatalf("contexts=%v shadowed=%v", saved.Contexts, saved.Shadowed)
	}
	if fi, _ := os.Stat(saved.Path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("file perms %v", fi.Mode().Perm())
	}
	if di, _ := os.Stat(filepath.Dir(saved.Path)); di.Mode().Perm() != 0o700 {
		t.Fatalf("dir perms %v", di.Mode().Perm())
	}
	if !slices.Contains(names(m), "team-a") {
		t.Fatal("pasted context not visible")
	}
	if !slices.ContainsFunc(m.Files(), func(f KubeconfigFile) bool { return f.Path == saved.Path && f.Managed && f.Removable }) {
		t.Fatal("pasted file not listed as managed/removable")
	}

	again, err := m.SaveKubeconfig("team a/../x", pasted) // hostile name gets sanitized
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(again.Path) != filepath.Dir(saved.Path) || strings.Contains(filepath.Base(again.Path), "/") {
		t.Fatalf("name not sanitized: %s", again.Path)
	}

	for _, bad := range []string{"", "not: [yaml", "apiVersion: v1\nkind: Config\n",
		"apiVersion: v1\nkind: Config\ncontexts: [{name: x, context: {cluster: missing, user: missing}}]\n"} {
		if _, err := m.SaveKubeconfig("", bad); err == nil {
			t.Fatalf("accepted invalid content %q", bad)
		}
	}

	if err := m.RemovePath(saved.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(saved.Path); !os.IsNotExist(err) {
		t.Fatal("removing a pasted kubeconfig must delete its file (credentials)")
	}
}
