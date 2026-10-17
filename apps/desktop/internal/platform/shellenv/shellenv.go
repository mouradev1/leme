// Package shellenv fixes the environment of GUI-launched apps.
//
// Apps started from Finder/Dock/a .desktop file inherit a minimal PATH, so
// kubeconfig exec credential plugins (aws, gke-gcloud-auth-plugin, kubelogin,
// oidc-login...) are "not found" even though they work in the terminal. We run
// the user's login shell once at startup and import its environment.
package shellenv

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const marker = "__LEME_ENV_MARKER__"

var skip = map[string]bool{"PWD": true, "OLDPWD": true, "SHLVL": true, "_": true}

func Load(timeout time.Duration) error {
	if runtime.GOOS == "windows" || os.Getenv("TERM_PROGRAM") != "" {
		return nil // Windows has no login-shell PATH problem; terminal launches already have the env.
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
		if runtime.GOOS == "darwin" {
			shell = "/bin/zsh"
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Markers isolate env output from anything .zshrc/.bashrc prints.
	cmd := exec.CommandContext(ctx, shell, "-ilc", "printf %s "+marker+"; env -0; printf %s "+marker)
	cmd.Stdin = nil
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	parts := bytes.Split(out, []byte(marker))
	if len(parts) < 3 {
		return nil
	}
	for _, kv := range bytes.Split(parts[1], []byte{0}) {
		k, v, ok := strings.Cut(string(kv), "=")
		if ok && k != "" && !skip[k] {
			_ = os.Setenv(k, v)
		}
	}
	return nil
}
