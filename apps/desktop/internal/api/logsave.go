package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"leme/internal/stream"
)

// SavedLogs tells the UI where the file went (Path "" = user cancelled).
type SavedLogs struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
}

// SaveLogs asks where to save, then downloads the logs straight from the
// cluster into that file (complete, or the last TailLines per source). The
// file is readable only by the current user.
func (a *StreamAPI) SaveLogs(cluster string, opts stream.LogOptions, fileName string) (SavedLogs, error) {
	if len(opts.Targets) == 0 {
		return SavedLogs{}, fmt.Errorf("no log targets")
	}
	s, err := a.d.session(cluster)
	if err != nil {
		return SavedLogs{}, err
	}
	path, err := a.pickSavePath(fileName)
	if err != nil || path == "" {
		return SavedLogs{}, err
	}
	f, err := stream.CreatePrivate(path)
	if err != nil {
		return SavedLogs{}, err
	}
	ctx, cancel := context.WithTimeout(a.d.Bus.Context(), 10*time.Minute)
	defer cancel()
	n, werr := stream.WriteLogs(ctx, s.Kube, opts, f)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return SavedLogs{Path: path, Lines: n}, werr
}

// SaveText asks where to save and writes text there (the lines on screen).
func (a *StreamAPI) SaveText(fileName, content string) (string, error) {
	path, err := a.pickSavePath(fileName)
	if err != nil || path == "" {
		return "", err
	}
	f, err := stream.CreatePrivate(path)
	if err != nil {
		return "", err
	}
	_, werr := f.WriteString(content)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return path, werr
}

func (a *StreamAPI) pickSavePath(fileName string) (string, error) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Downloads")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		dir = home
	}
	return runtime.SaveFileDialog(a.d.Bus.Context(), runtime.SaveDialogOptions{
		Title:                "Salvar logs",
		DefaultDirectory:     dir,
		DefaultFilename:      safeFileName(fileName),
		CanCreateDirectories: true,
		Filters:              []runtime.FileFilter{{DisplayName: "Logs (*.log, *.txt)", Pattern: "*.log;*.txt"}},
	})
}

// safeFileName keeps a suggested name to plain characters (no paths).
func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '-'
	}, filepath.Base(s))
	s = strings.Trim(s, ".-")
	if s == "" {
		return "logs.log"
	}
	if len(s) > 120 {
		s = s[:120]
	}
	if !strings.HasSuffix(s, ".log") && !strings.HasSuffix(s, ".txt") {
		s += ".log"
	}
	return s
}
