package api

import (
	"errors"
	"leme/internal/platform/i18n"

	"leme/internal/platform/settings"
	"leme/internal/terminal"
)

type TerminalAPI struct{ d *Deps }

// OpenExec opens a shell in a container. Output: "term:<id>:data" (base64),
// end: "term:<id>:exit".
func (a *TerminalAPI) OpenExec(id, cluster string, target terminal.ExecTarget, cols, rows uint16) error {
	if err := a.d.writable(cluster); err != nil {
		return err
	}
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	// An open shell keeps the cluster session alive until it exits.
	start := terminal.Exec(s, target, cols, rows)
	s.Acquire()
	err = a.d.Terminals.Open(id, func(out func([]byte), done func(error)) (terminal.Session, error) {
		return start(out, func(e error) { s.Release(); done(e) })
	})
	if err != nil {
		s.Release()
	}
	return err
}

// OpenNodeShell opens a root shell on a node through a temporary privileged
// pod (deleted when the shell closes). The UI asks for confirmation first.
func (a *TerminalAPI) OpenNodeShell(id, cluster, node string, cols, rows uint16) error {
	if err := a.d.writable(cluster); err != nil {
		return err
	}
	s, err := a.d.session(cluster)
	if err != nil {
		return err
	}
	st := settings.Load()
	if st.NodeShellDisabled {
		return errors.New(i18n.T(i18n.T("o shell no node está desligado nas configurações do Leme", "the node shell is turned off in Leme's settings"), "the node shell is turned off in Leme's settings"))
	}
	image := terminal.DefaultNodeShellImage
	if st.NodeShellImage != "" {
		image = st.NodeShellImage
	}
	start := terminal.NodeShell(s, node, image, cols, rows)
	s.Acquire()
	err = a.d.Terminals.Open(id, func(out func([]byte), done func(error)) (terminal.Session, error) {
		return start(out, func(e error) { s.Release(); done(e) })
	})
	if err != nil {
		s.Release()
	}
	return err
}

// OpenLocal opens a host shell (PTY) pre-wired to the given cluster.
func (a *TerminalAPI) OpenLocal(id, cluster string, cols, rows uint16) error {
	if err := a.d.writable(cluster); err != nil {
		return err
	}
	kubeconfig, err := a.d.Clusters.ScopedKubeconfig(cluster)
	if err != nil {
		return err
	}
	return a.d.Terminals.Open(id, terminal.Local(kubeconfig, cols, rows))
}

func (a *TerminalAPI) Write(id, data string) error { return a.d.Terminals.Write(id, []byte(data)) }

func (a *TerminalAPI) Resize(id string, cols, rows uint16) error {
	return a.d.Terminals.Resize(id, cols, rows)
}

func (a *TerminalAPI) Close(id string) { a.d.Terminals.Close(id) }
