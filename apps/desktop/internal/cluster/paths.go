package cluster

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

// KubeconfigFile describes one file feeding the merged context list.
type KubeconfigFile struct {
	Path      string `json:"path"`
	Contexts  int    `json:"contexts"`
	Exists    bool   `json:"exists"`
	Removable bool   `json:"removable"` // false for $KUBECONFIG / ~/.kube/config
	Managed   bool   `json:"managed"`   // pasted into the app (stored in its private dir)
}

func newRules(extra []string) *clientcmd.ClientConfigLoadingRules {
	r := clientcmd.NewDefaultClientConfigLoadingRules()
	for _, p := range extra {
		if !slices.Contains(r.Precedence, p) {
			r.Precedence = append(r.Precedence, p)
		}
	}
	return r
}

func normalize(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[1:])
	}
	return filepath.Abs(p)
}

// precedence returns the absolute paths of every kubeconfig file in use.
func (m *Manager) precedence() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	for _, p := range m.rules.GetLoadingPrecedence() {
		if abs, err := filepath.Abs(p); err == nil {
			out = append(out, abs)
		}
	}
	return out
}

// Files lists every kubeconfig file and how many contexts it contributed.
// When two files define the same context name, the first file wins (same
// merge semantics as kubectl), so the count may be lower than the file's own.
func (m *Manager) Files() []KubeconfigFile {
	paths := m.precedence()
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]KubeconfigFile, 0, len(paths))
	for _, p := range paths {
		f := KubeconfigFile{Path: p, Removable: slices.Contains(m.extra, p), Managed: isManaged(p)}
		_, err := os.Stat(p)
		f.Exists = err == nil
		for _, c := range m.raw.Contexts {
			if c.LocationOfOrigin == p {
				f.Contexts++
			}
		}
		out = append(out, f)
	}
	return out
}

// ExtraPaths is what gets persisted in settings.
func (m *Manager) ExtraPaths() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return slices.Clone(m.extra)
}

// AddPath validates a kubeconfig file located anywhere (another folder, a
// mounted drive, a file from a teammate) and merges its contexts.
// Returns how many contexts the file defines.
func (m *Manager) AddPath(path string) (int, error) {
	abs, err := normalize(path)
	if err != nil {
		return 0, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return 0, fmt.Errorf("arquivo não encontrado: %s", abs)
	}
	if st.IsDir() {
		return 0, fmt.Errorf("%s é uma pasta; selecione o arquivo kubeconfig", abs)
	}
	cfg, err := clientcmd.LoadFromFile(abs)
	if err != nil {
		return 0, fmt.Errorf("não é um kubeconfig válido: %w", err)
	}
	if len(cfg.Contexts) == 0 {
		return 0, fmt.Errorf("%s não define nenhum context", abs)
	}

	if !slices.Contains(m.precedence(), abs) {
		m.mu.Lock()
		m.extra = append(m.extra, abs)
		m.rules = newRules(m.extra)
		m.mu.Unlock()
		if m.watcher != nil {
			_ = m.watcher.Add(filepath.Dir(abs))
		}
	}
	return len(cfg.Contexts), m.Reload()
}

// RemovePath drops a user-added file. Sessions of contexts that disappear are
// closed by Reload (their fingerprint no longer matches).
func (m *Manager) RemovePath(path string) error {
	abs, err := normalize(path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	i := slices.Index(m.extra, abs)
	if i < 0 {
		m.mu.Unlock()
		return fmt.Errorf("%s não foi adicionado pelo app (vem de $KUBECONFIG ou ~/.kube/config)", abs)
	}
	m.extra = slices.Delete(m.extra, i, i+1)
	m.rules = newRules(m.extra)
	m.mu.Unlock()
	if isManaged(abs) {
		_ = os.Remove(abs) // pasted by the user into the app: don't leave credentials behind
	}
	return m.Reload()
}

// SavedKubeconfig is the result of SaveKubeconfig.
type SavedKubeconfig struct {
	Path     string   `json:"path"`
	Contexts []string `json:"contexts"`
	// Shadowed: context names that already existed in another file. kubectl
	// merge rules keep the first one, so these entries won't be the new ones.
	Shadowed []string `json:"shadowed"`
}

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// managedDir holds kubeconfigs pasted into the app. They contain credentials:
// private dir (0700), files 0600, deleted when removed from the app.
func managedDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "leme", "kubeconfigs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, os.Chmod(dir, 0o700)
}

func isManaged(path string) bool {
	dir, err := managedDir()
	return err == nil && filepath.Dir(path) == dir
}

// SaveKubeconfig validates pasted kubeconfig YAML, stores it in the app's
// private kubeconfig dir and adds it, so the clusters show up right away.
// name is optional (defaults to the current-context / first context).
func (m *Manager) SaveKubeconfig(name, content string) (SavedKubeconfig, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return SavedKubeconfig{}, fmt.Errorf("cole o conteúdo do kubeconfig")
	}
	cfg, err := clientcmd.Load([]byte(content))
	if err != nil {
		return SavedKubeconfig{}, fmt.Errorf("conteúdo não é um kubeconfig válido: %w", err)
	}
	if len(cfg.Contexts) == 0 {
		return SavedKubeconfig{}, fmt.Errorf("o kubeconfig não tem nenhum context")
	}
	// Same checks kubectl runs: every context points to an existing cluster and
	// user, cert/key files referenced by path exist, exec plugins are complete.
	if err := clientcmd.Validate(*cfg); err != nil {
		return SavedKubeconfig{}, fmt.Errorf("kubeconfig incompleto: %w", err)
	}

	out := SavedKubeconfig{}
	m.mu.RLock()
	for ctx := range cfg.Contexts {
		out.Contexts = append(out.Contexts, ctx)
		if _, exists := m.raw.Contexts[ctx]; exists {
			out.Shadowed = append(out.Shadowed, ctx)
		}
	}
	m.mu.RUnlock()
	slices.Sort(out.Contexts)
	slices.Sort(out.Shadowed)

	if strings.TrimSpace(name) == "" {
		name = cfg.CurrentContext
		if name == "" {
			name = out.Contexts[0]
		}
	}
	slug := strings.Trim(unsafeName.ReplaceAllString(name, "-"), "-.")
	if slug == "" {
		slug = "cluster"
	}
	dir, err := managedDir()
	if err != nil {
		return SavedKubeconfig{}, err
	}
	path := filepath.Join(dir, slug+".yaml")
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s-%d.yaml", slug, i))
	}
	// O_EXCL: never clobber a file created between the Stat and the write.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return SavedKubeconfig{}, err
	}
	if _, err := f.WriteString(content + "\n"); err != nil {
		f.Close()
		_ = os.Remove(path)
		return SavedKubeconfig{}, err
	}
	if err := f.Close(); err != nil {
		return SavedKubeconfig{}, err
	}
	if _, err := m.AddPath(path); err != nil {
		_ = os.Remove(path)
		return SavedKubeconfig{}, err
	}
	out.Path = path
	return out, nil
}
