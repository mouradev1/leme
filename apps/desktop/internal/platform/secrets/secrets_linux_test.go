//go:build linux

package secrets

import (
	"errors"
	"os"
	"testing"
)

// Needs a running Secret Service: LEME_SECRET_SERVICE_TEST=1 go test ./internal/platform/secrets/
func TestSecretServiceRoundTrip(t *testing.T) {
	if os.Getenv("LEME_SECRET_SERVICE_TEST") == "" {
		t.Skip("set LEME_SECRET_SERVICE_TEST=1 with a Secret Service running")
	}
	if !secretServiceAvailable() {
		t.Fatal("no Secret Service on the session bus")
	}
	s := secretService{}
	acct := "https://leme-test.invalid"
	t.Cleanup(func() { _ = s.Delete(acct) })
	if err := s.Set(acct, "token-abc:xyz"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(acct); err != nil || got != "token-abc:xyz" {
		t.Fatalf("got %q, %v", got, err)
	}
	if err := s.Set(acct, "token-abc:new"); err != nil { // replace, not duplicate
		t.Fatal(err)
	}
	if got, _ := s.Get(acct); got != "token-abc:new" {
		t.Fatalf("after replace: %q", got)
	}
	if err := s.Delete(acct); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(acct); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}
