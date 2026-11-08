//go:build linux

package secrets

import (
	"errors"
	"fmt"
	"leme/internal/platform/i18n"
	"time"

	"github.com/godbus/dbus/v5"
)

// Freedesktop Secret Service (GNOME Keyring, KWallet 5.97+, KeePassXC) over
// the user's D-Bus session. Items carry the attributes service=Leme and
// account=<account>, so they show up in Seahorse ("Passwords and Keys").
//
// The secret travels on the private session bus with the "plain" algorithm,
// like most libraries do: only processes of the same user can reach that bus.

const (
	ssName      = "org.freedesktop.secrets"
	ssPath      = dbus.ObjectPath("/org/freedesktop/secrets")
	ssService   = "org.freedesktop.Secret.Service"
	ssColl      = "org.freedesktop.Secret.Collection"
	ssItem      = "org.freedesktop.Secret.Item"
	ssPrompt    = "org.freedesktop.Secret.Prompt"
	ssSession   = "org.freedesktop.Secret.Session"
	noPrompt    = dbus.ObjectPath("/")
	promptLimit = 2 * time.Minute // time to type the keyring password
)

// ssSecret is the Secret Service (oayays) struct.
type ssSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

type secretService struct{}

// platformStore uses the Secret Service when one answers on the session bus;
// otherwise (no desktop session, WSL) the private file.
func platformStore() Store {
	if secretServiceAvailable() {
		return secretService{}
	}
	return &fileStore{}
}

func secretServiceAvailable() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer conn.Close()
	done := make(chan error, 1)
	go func() { done <- conn.Object(ssName, ssPath).Call("org.freedesktop.DBus.Peer.Ping", 0).Err }() // also starts an activatable keyring
	select {
	case err := <-done:
		return err == nil
	case <-time.After(3 * time.Second):
		return false
	}
}

// open connects and starts a Secret Service session; close with done().
func open() (conn *dbus.Conn, session dbus.ObjectPath, done func(), err error) {
	conn, err = dbus.ConnectSessionBus()
	if err != nil {
		return nil, "", nil, fmt.Errorf("secret service: %w", err)
	}
	var out dbus.Variant
	if err = conn.Object(ssName, ssPath).Call(ssService+".OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&out, &session); err != nil {
		conn.Close()
		return nil, "", nil, fmt.Errorf("secret service: %w", err)
	}
	done = func() {
		conn.Object(ssName, session).Call(ssSession+".Close", 0)
		conn.Close()
	}
	return conn, session, done, nil
}

func attrs(account string) map[string]string {
	return map[string]string{"service": Service, "account": account}
}

// prompt runs a Secret Service prompt (e.g. "unlock your keyring") and waits.
func prompt(conn *dbus.Conn, p dbus.ObjectPath) error {
	if p == noPrompt || p == "" {
		return nil
	}
	match := []dbus.MatchOption{dbus.WithMatchObjectPath(p), dbus.WithMatchInterface(ssPrompt), dbus.WithMatchMember("Completed")}
	if err := conn.AddMatchSignal(match...); err != nil {
		return err
	}
	defer conn.RemoveMatchSignal(match...)
	ch := make(chan *dbus.Signal, 4)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)
	if err := conn.Object(ssName, p).Call(ssPrompt+".Prompt", 0, "").Err; err != nil {
		return err
	}
	timeout := time.After(promptLimit)
	for {
		select {
		case s := <-ch:
			if s == nil || s.Path != p || s.Name != ssPrompt+".Completed" {
				continue
			}
			if len(s.Body) > 0 {
				if dismissed, ok := s.Body[0].(bool); ok && dismissed {
					return errors.New(i18n.T("secret service: o desbloqueio do cofre foi cancelado", "secret service: unlocking the keyring was cancelled"))
				}
			}
			return nil
		case <-timeout:
			return errors.New(i18n.T("secret service: o cofre não foi desbloqueado a tempo", "secret service: the keyring was not unlocked in time"))
		}
	}
}

func unlock(conn *dbus.Conn, paths []dbus.ObjectPath) error {
	if len(paths) == 0 {
		return nil
	}
	var unlocked []dbus.ObjectPath
	var p dbus.ObjectPath
	if err := conn.Object(ssName, ssPath).Call(ssService+".Unlock", 0, paths).Store(&unlocked, &p); err != nil {
		return err
	}
	return prompt(conn, p)
}

// find returns the (unlocked) items for an account.
func find(conn *dbus.Conn, account string) ([]dbus.ObjectPath, error) {
	var unlocked, locked []dbus.ObjectPath
	if err := conn.Object(ssName, ssPath).Call(ssService+".SearchItems", 0, attrs(account)).Store(&unlocked, &locked); err != nil {
		return nil, err
	}
	if err := unlock(conn, locked); err != nil {
		return nil, err
	}
	return append(unlocked, locked...), nil
}

func (secretService) Get(account string) (string, error) {
	if account == "" {
		return "", fmt.Errorf("invalid account name")
	}
	conn, session, done, err := open()
	if err != nil {
		return "", err
	}
	defer done()
	items, err := find(conn, account)
	if err != nil {
		return "", fmt.Errorf("secret service: %w", err)
	}
	if len(items) == 0 {
		return "", ErrNotFound
	}
	var s ssSecret
	if err := conn.Object(ssName, items[0]).Call(ssItem+".GetSecret", 0, session).Store(&s); err != nil {
		return "", fmt.Errorf("secret service: %w", err)
	}
	return string(s.Value), nil
}

func (secretService) Set(account, secret string) error {
	if account == "" || secret == "" {
		return fmt.Errorf("invalid account or secret")
	}
	conn, session, done, err := open()
	if err != nil {
		return err
	}
	defer done()
	var coll dbus.ObjectPath
	if err := conn.Object(ssName, ssPath).Call(ssService+".ReadAlias", 0, "default").Store(&coll); err != nil {
		return fmt.Errorf("secret service: %w", err)
	}
	if coll == noPrompt || coll == "" {
		return errors.New(i18n.T("secret service: não há um cofre padrão (crie um chaveiro \"Login\" no gerenciador de senhas)", "secret service: there is no default keyring (create a \"Login\" keyring in the password manager)"))
	}
	if err := unlock(conn, []dbus.ObjectPath{coll}); err != nil {
		return fmt.Errorf("secret service: %w", err)
	}
	props := map[string]dbus.Variant{
		ssItem + ".Label":      dbus.MakeVariant(Service + " · " + account),
		ssItem + ".Attributes": dbus.MakeVariant(attrs(account)),
	}
	s := ssSecret{Session: session, Parameters: []byte{}, Value: []byte(secret), ContentType: "text/plain; charset=utf8"}
	var item, p dbus.ObjectPath
	if err := conn.Object(ssName, coll).Call(ssColl+".CreateItem", 0, props, s, true).Store(&item, &p); err != nil {
		return fmt.Errorf("secret service: %w", err)
	}
	return prompt(conn, p)
}

func (secretService) Delete(account string) error {
	if account == "" {
		return fmt.Errorf("invalid account name")
	}
	conn, _, done, err := open()
	if err != nil {
		return err
	}
	defer done()
	items, err := find(conn, account)
	if err != nil {
		return fmt.Errorf("secret service: %w", err)
	}
	for _, it := range items {
		var p dbus.ObjectPath
		if err := conn.Object(ssName, it).Call(ssItem+".Delete", 0).Store(&p); err != nil {
			return fmt.Errorf("secret service: %w", err)
		}
		if err := prompt(conn, p); err != nil {
			return err
		}
	}
	return nil
}
