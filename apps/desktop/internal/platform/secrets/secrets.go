// Package secrets keeps credentials (Rancher tokens) in the operating
// system's password store instead of in Leme's settings file.
//
//   - macOS: the login Keychain, through Apple's /usr/bin/security tool. The
//     secret is sent on its stdin, never as a command-line argument (which
//     other processes could see).
//   - Windows: Credential Manager (the store Git and Docker use), through the
//     Win32 API in advapi32.dll.
//   - Linux: the Secret Service (GNOME Keyring, KWallet) over D-Bus. Without
//     a running service (server, WSL) it falls back to a file only the current
//     user can read, in Leme's config dir.
package secrets

import (
	"errors"
	"strings"
)

// Service names Leme's entries in the password store.
const Service = "Leme"

// ErrNotFound: nothing stored for that account.
var ErrNotFound = errors.New("secret not found")

// Store reads and writes secrets by account name (e.g. a Rancher URL).
type Store interface {
	Get(account string) (string, error)
	Set(account, secret string) error
	Delete(account string) error
}

// Default is the store for this operating system.
func Default() Store { return platformStore() }

// safe rejects values that could break out of a quoted argument.
func safe(v string) bool {
	return v != "" && !strings.ContainsAny(v, "\"\\\n\r'")
}

// Memory is an in-memory Store for tests.
type Memory map[string]string

func (m Memory) Get(a string) (string, error) {
	if v, ok := m[a]; ok {
		return v, nil
	}
	return "", ErrNotFound
}
func (m Memory) Set(a, s string) error { m[a] = s; return nil }
func (m Memory) Delete(a string) error { delete(m, a); return nil }
