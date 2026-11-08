//go:build windows

package secrets

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows Credential Manager (Control Panel > Credential Manager > Windows
// Credentials), entries named "Leme:<account>". Protected by the user's
// Windows login.
var (
	advapi32      = windows.NewLazySystemDLL("advapi32.dll")
	procCredWrite = advapi32.NewProc("CredWriteW")
	procCredRead  = advapi32.NewProc("CredReadW")
	procCredDel   = advapi32.NewProc("CredDeleteW")
	procCredFree  = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2       // survives logoff, stays on this computer
	credMaxBlob             = 5 * 512 // CRED_MAX_CREDENTIAL_BLOB_SIZE
)

// credential mirrors the Win32 CREDENTIALW struct.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type credManager struct{}

func platformStore() Store { return credManager{} }

func target(account string) (*uint16, error) {
	if account == "" {
		return nil, fmt.Errorf("invalid account name")
	}
	return windows.UTF16PtrFromString(Service + ":" + account)
}

func (credManager) Get(account string) (string, error) {
	t, err := target(account)
	if err != nil {
		return "", err
	}
	var c *credential
	r, _, callErr := procCredRead.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&c)))
	if r == 0 {
		if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("credential manager: %w", callErr)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(c)))
	if c.CredentialBlobSize == 0 || c.CredentialBlob == nil {
		return "", ErrNotFound
	}
	return string(unsafe.Slice(c.CredentialBlob, c.CredentialBlobSize)), nil // string() copies before CredFree
}

func (credManager) Set(account, secret string) error {
	t, err := target(account)
	if err != nil {
		return err
	}
	if secret == "" || len(secret) > credMaxBlob {
		return fmt.Errorf("invalid secret size")
	}
	user, err := windows.UTF16PtrFromString(account)
	if err != nil {
		return err
	}
	blob := []byte(secret)
	c := credential{
		Type: credTypeGeneric, TargetName: t, UserName: user, Persist: credPersistLocalMachine,
		CredentialBlobSize: uint32(len(blob)), CredentialBlob: &blob[0],
	}
	if r, _, callErr := procCredWrite.Call(uintptr(unsafe.Pointer(&c)), 0); r == 0 {
		return fmt.Errorf("credential manager: %w", callErr)
	}
	return nil
}

func (credManager) Delete(account string) error {
	t, err := target(account)
	if err != nil {
		return err
	}
	if r, _, callErr := procCredDel.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0); r == 0 {
		if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return nil
		}
		return fmt.Errorf("credential manager: %w", callErr)
	}
	return nil
}
