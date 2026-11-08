package api

import (
	"path/filepath"
	"testing"

	"leme/internal/platform/settings"
	"leme/internal/resources"
)

func TestReadOnlyGuard(t *testing.T) {
	home := t.TempDir() // never touch the real settings
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	a := &ClusterAPI{d: &Deps{}}
	if err := a.d.writable("prod"); err != nil {
		t.Fatal("unmarked cluster must be writable")
	}
	if err := a.SetClusterPrefs("prod", settings.ClusterPrefs{Production: true, ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.d.writable("prod"); err == nil {
		t.Fatal("read-only cluster accepted a change")
	}
	if !a.ClusterPrefs()["prod"].Production {
		t.Fatal("production mark not saved")
	}
	// Dry-run validation stays allowed; a real apply is refused before any network call.
	r := &ResourceAPI{d: a.d}
	if _, err := r.Apply("prod", "kind: x", resourcesApplyOpts(false)); err == nil {
		t.Fatal("apply on read-only cluster")
	}
	if err := a.SetClusterPrefs("prod", settings.ClusterPrefs{}); err != nil || len(a.ClusterPrefs()) != 0 {
		t.Fatal("clearing the marks must remove the entry")
	}
}

func resourcesApplyOpts(dry bool) resources.ApplyOptions { return resources.ApplyOptions{DryRun: dry} }
