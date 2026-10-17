//go:build windows

package terminal

import "errors"

// Local on Windows needs ConPTY (e.g. github.com/UserExistsError/conpty or
// golang.org/x/sys/windows CreatePseudoConsole). Same Session contract.
func Local(kubeconfig string, cols, rows uint16) Starter {
	return func(out func([]byte), done func(error)) (Session, error) {
		return nil, errors.New("local terminal: ConPTY backend not implemented yet")
	}
}
