//go:build !windows

package terminal

import (
	"errors"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// Local starts the user's login shell on a real host PTY, with KUBECONFIG
// pointing at a single-context kubeconfig so `kubectl` and `helm` typed in
// this tab always target the cluster the tab belongs to.
func Local(kubeconfig string, cols, rows uint16) Starter {
	return func(out func([]byte), done func(error)) (Session, error) {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		cmd := exec.Command(shell, "-l")
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig, "TERM=xterm-256color", "COLORTERM=truecolor")
		cmd.Dir, _ = os.UserHomeDir()

		f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
		if err != nil {
			return nil, err
		}
		ls := &localSession{f: f, cmd: cmd, kubeconfig: kubeconfig}

		go func() {
			buf := make([]byte, 32<<10)
			for {
				n, err := f.Read(buf)
				if n > 0 {
					out(buf[:n])
				}
				if err != nil {
					break // EIO on Linux / EOF on macOS when the shell exits
				}
			}
			werr := cmd.Wait()
			_ = f.Close()
			_ = os.Remove(kubeconfig)
			var exitErr *exec.ExitError
			if errors.As(werr, &exitErr) && exitErr.ExitCode() >= 0 {
				werr = nil // shell exited with a status (e.g. `exit 1`): not an app error
			}
			done(werr)
		}()
		return ls, nil
	}
}

type localSession struct {
	f          *os.File
	cmd        *exec.Cmd
	kubeconfig string
}

func (l *localSession) Write(p []byte) error { _, err := l.f.Write(p); return err }

func (l *localSession) Resize(cols, rows uint16) error {
	return pty.Setsize(l.f, &pty.Winsize{Cols: cols, Rows: rows}) // kernel sends SIGWINCH
}

func (l *localSession) Close() error {
	if l.cmd.Process != nil {
		_ = l.cmd.Process.Signal(os.Interrupt)
		_ = l.cmd.Process.Kill()
	}
	return l.f.Close()
}
