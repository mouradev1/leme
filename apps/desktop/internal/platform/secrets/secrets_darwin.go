package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const securityTool = "/usr/bin/security"

type keychain struct{}

func platformStore() Store { return keychain{} }

func (keychain) Get(account string) (string, error) {
	if !safe(account) {
		return "", fmt.Errorf("invalid account name")
	}
	out, err := exec.Command(securityTool, "find-generic-password", "-s", Service, "-a", account, "-w").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 44 { // errSecItemNotFound
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keychain: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// Set runs `security -i` and passes the command on stdin, so the secret never
// shows up in the process list.
func (k keychain) Set(account, secret string) error {
	if !safe(account) || !safe(secret) {
		return fmt.Errorf("invalid account or secret")
	}
	cmd := exec.Command(securityTool, "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("add-generic-password -U -s \"%s\" -a \"%s\" -l \"%s\" -w \"%s\"\n", Service, account, Service+" · "+account, secret))
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stderr, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("keychain: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	// `security -i` exits 0 even when a command fails: read it back.
	if got, err := k.Get(account); err != nil || got != secret {
		return fmt.Errorf("keychain: could not store the secret: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (keychain) Delete(account string) error {
	if !safe(account) {
		return fmt.Errorf("invalid account name")
	}
	err := exec.Command(securityTool, "delete-generic-password", "-s", Service, "-a", account).Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 44 {
		return nil
	}
	return err
}
