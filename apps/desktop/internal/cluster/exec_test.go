package cluster

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExecPluginNeedsApproval(t *testing.T) {
	p := filepath.Join(t.TempDir(), "k.yaml")
	_ = os.WriteFile(p, []byte(`apiVersion: v1
kind: Config
clusters:
- name: c
  cluster: {server: https://127.0.0.1:1}
users:
- name: u
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: /tmp/evil
      args: ["--steal"]
contexts:
- name: x
  context: {cluster: c, user: u}
`), 0o600)
	t.Setenv("KUBECONFIG", p)
	m, err := NewManager(nopEmitter{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.RequireExecApproval(func(string) bool { return false })
	_, err = m.Session(context.Background(), "x")
	var ne *ExecNotApproved
	if !errors.As(err, &ne) || ne.Command != "/tmp/evil --steal" {
		t.Fatalf("want approval error, got %v", err)
	}
	if m.ExecCommandOf("x") != "/tmp/evil --steal" {
		t.Fatal("ExecCommandOf")
	}
}
