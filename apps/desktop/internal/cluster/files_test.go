package cluster

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func kubeconfig(ctxs ...string) string {
	s := "apiVersion: v1\nkind: Config\nclusters:\n- name: c\n  cluster: {server: https://x}\nusers:\n- name: u\n  user: {token: t}\ncontexts:\n"
	for _, c := range ctxs {
		s += "- name: " + c + "\n  context: {cluster: c, user: u}\n"
	}
	return s
}

func TestFilesListNamesAndShadowed(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.yaml"), filepath.Join(dir, "b.yaml")
	_ = os.WriteFile(a, []byte(kubeconfig("prod", "dev")), 0o600)
	_ = os.WriteFile(b, []byte(kubeconfig("dev", "hml")), 0o600)
	t.Setenv("KUBECONFIG", a)
	m, err := NewManager(nopEmitter{}, b)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	files := m.Files()
	if len(files) != 2 {
		t.Fatalf("files: %+v", files)
	}
	if !slices.Equal(files[0].Names, []string{"dev", "prod"}) || len(files[0].Shadowed) != 0 {
		t.Fatalf("first file: %+v", files[0])
	}
	if !slices.Equal(files[1].Names, []string{"hml"}) || !slices.Equal(files[1].Shadowed, []string{"dev"}) {
		t.Fatalf("second file: %+v", files[1])
	}
}
