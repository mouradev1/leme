//go:build !darwin

package secrets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// fileStore is the fallback when the system has no password store (a Linux
// server without a desktop session, WSL): a JSON file only the current user
// can read, in Leme's config dir.
type fileStore struct{ mu sync.Mutex }

func path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "leme", "secrets.json"), nil
}

func (f *fileStore) load() (map[string]string, string, error) {
	p, err := path()
	if err != nil {
		return nil, "", err
	}
	m := map[string]string{}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m, p, nil
}

func (f *fileStore) save(m map[string]string, p string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(m)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (f *fileStore) Get(a string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, _, err := f.load()
	if err != nil {
		return "", err
	}
	if v, ok := m[a]; ok {
		return v, nil
	}
	return "", ErrNotFound
}

func (f *fileStore) Set(a, s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, p, err := f.load()
	if err != nil {
		return err
	}
	m[a] = s
	return f.save(m, p)
}

func (f *fileStore) Delete(a string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, p, err := f.load()
	if err != nil {
		return err
	}
	delete(m, a)
	return f.save(m, p)
}
