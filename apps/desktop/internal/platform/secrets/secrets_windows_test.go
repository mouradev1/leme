//go:build windows

package secrets

import (
	"errors"
	"testing"
	"unsafe"
)

func TestCredentialLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) == 8 && unsafe.Sizeof(credential{}) != 80 { // sizeof(CREDENTIALW) on 64-bit
		t.Fatalf("credential struct is %d bytes, want 80", unsafe.Sizeof(credential{}))
	}
}

func TestCredManagerRoundTrip(t *testing.T) {
	s := credManager{}
	acct := "https://leme-test.invalid"
	t.Cleanup(func() { _ = s.Delete(acct) })
	if err := s.Set(acct, "token-abc:xyz"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(acct); err != nil || got != "token-abc:xyz" {
		t.Fatalf("got %q, %v", got, err)
	}
	if err := s.Delete(acct); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(acct); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}
