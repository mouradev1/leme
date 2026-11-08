package secrets

import (
	"errors"
	"os"
	"testing"
)

// Touches the real login Keychain, so it only runs when asked:
// LEME_KEYCHAIN_TEST=1 go test ./internal/platform/secrets/
func TestKeychainRoundTrip(t *testing.T) {
	if os.Getenv("LEME_KEYCHAIN_TEST") == "" {
		t.Skip("set LEME_KEYCHAIN_TEST=1 to run against the login Keychain")
	}
	k := keychain{}
	acct := "https://leme-test.invalid"
	t.Cleanup(func() { _ = k.Delete(acct) })
	if err := k.Set(acct, "token-abc:xyz"); err != nil {
		t.Fatal(err)
	}
	if got, err := k.Get(acct); err != nil || got != "token-abc:xyz" {
		t.Fatalf("got %q, %v", got, err)
	}
	if err := k.Set(acct, "token-abc:new"); err != nil { // update in place
		t.Fatal(err)
	}
	if err := k.Delete(acct); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get(acct); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestSafeRejectsQuotes(t *testing.T) {
	for _, v := range []string{"", `a"b`, `a\b`, "a\nb", "a'b"} {
		if safe(v) {
			t.Errorf("safe(%q) = true", v)
		}
	}
}
