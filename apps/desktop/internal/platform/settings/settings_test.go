package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(base, "btu")
	pasted := filepath.Join(oldDir, "kubeconfigs", "team.yaml")
	if err := os.MkdirAll(filepath.Dir(pasted), 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(pasted, []byte("apiVersion: v1\n"), 0o600)
	external := "/somewhere/else/config"
	// Settings written by the old version, in the old dir.
	_ = os.WriteFile(filepath.Join(oldDir, "settings.json"),
		[]byte(`{"kubeconfigPaths":["`+pasted+`","`+external+`"]}`), 0o600)

	if err := MigrateLegacy(); err != nil {
		t.Fatal(err)
	}
	newPasted := filepath.Join(base, "leme", "kubeconfigs", "team.yaml")
	if _, err := os.Stat(newPasted); err != nil {
		t.Fatal("pasted kubeconfig not moved")
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatal("legacy dir still there")
	}
	s := Load()
	if len(s.KubeconfigPaths) != 2 || s.KubeconfigPaths[0] != newPasted || s.KubeconfigPaths[1] != external {
		t.Fatalf("paths not rewritten: %v", s.KubeconfigPaths)
	}
	if err := MigrateLegacy(); err != nil { // idempotent
		t.Fatal(err)
	}
}
