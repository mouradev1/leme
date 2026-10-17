//go:build linux

// Package desktopinstall makes a downloaded Linux binary behave like an
// installed app: run it once from anywhere (Downloads, a USB stick...) and it
// copies itself to ~/.local/bin, installs its icon and a .desktop entry so it
// shows up in the app menu/launcher. No root, no package manager.
//
// Running a newer download later simply updates the installed copy. Copies
// under /usr or /opt (installed by the .deb) are left alone.
package desktopinstall

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type paths struct{ bin, icon, desktop string }

func locations() (paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return paths{}, err
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	return paths{
		bin:     filepath.Join(home, ".local", "bin", "leme"),
		icon:    filepath.Join(data, "icons", "hicolor", "512x512", "apps", "leme.png"),
		desktop: filepath.Join(data, "applications", "leme.desktop"),
	}, nil
}

// HandleFlags processes --uninstall. Returns true if the program should exit.
func HandleFlags() bool {
	if !slices.Contains(os.Args[1:], "--uninstall") {
		return false
	}
	p, err := locations()
	if err == nil {
		for _, f := range []string{p.bin, p.icon, p.desktop} {
			_ = os.Remove(f)
		}
		refresh(filepath.Dir(p.desktop))
	}
	fmt.Println("leme removido do menu de aplicativos e de ~/.local/bin")
	return true
}

// SelfInstall installs/updates the running binary as a desktop app. Returns
// true when it (re)installed. Skip with --no-install or LEME_NO_INSTALL=1.
func SelfInstall(icon []byte) (bool, error) {
	if os.Getenv("LEME_NO_INSTALL") != "" || slices.Contains(os.Args[1:], "--no-install") {
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return false, err
	}
	if strings.HasPrefix(exe, "/usr/") || strings.HasPrefix(exe, "/opt/") {
		return false, nil // system install (.deb): the package owns the menu entry
	}
	p, err := locations()
	if err != nil {
		return false, err
	}

	if exe != p.bin {
		if err := copyExecutable(exe, p.bin); err != nil {
			return false, fmt.Errorf("copiar binário: %w", err)
		}
	} else if _, err := os.Stat(p.desktop); err == nil {
		return false, nil // already installed and running the installed copy
	}

	removeLegacy(p)
	if err := writeFile(p.icon, icon, 0o644); err != nil {
		return false, err
	}
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Leme
GenericName=Kubernetes IDE
Comment=IDE desktop leve para Kubernetes
Exec="%s" %%U
Icon=%s
Terminal=false
Categories=Development;System;
Keywords=kubernetes;k8s;kubectl;helm;
StartupWMClass=leme
`, p.bin, p.icon)
	if err := writeFile(p.desktop, []byte(entry), 0o644); err != nil {
		return false, err
	}
	refresh(filepath.Dir(p.desktop))
	return true, nil
}

// copyExecutable writes to a temp file and renames, so updating the binary
// while an older copy is running never corrupts it ("text file busy").
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
}

// refresh nudges desktop environments to pick up the new entry right away.
func refresh(dir string) {
	if bin, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(bin, dir).Run()
	}
}

// removeLegacy deletes the menu entry, icon and binary installed under the
// app's pre-rename name ("btu"), so the launcher doesn't show both.
func removeLegacy(p paths) {
	for _, f := range []string{
		filepath.Join(filepath.Dir(p.bin), "btu"),
		filepath.Join(filepath.Dir(p.icon), "btu.png"),
		filepath.Join(filepath.Dir(p.desktop), "btu.desktop"),
	} {
		_ = os.Remove(f)
	}
}
