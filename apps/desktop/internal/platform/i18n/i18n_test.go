package i18n

import "testing"

func TestSwitch(t *testing.T) {
	t.Cleanup(func() { Set("pt") })
	if T("olá", "hello") != "olá" {
		t.Fatal("Portuguese is the default")
	}
	Set("en")
	if !English() || T("olá", "hello") != "hello" {
		t.Fatal("English not selected")
	}
	Set("xx")
	if T("olá", "hello") != "olá" {
		t.Fatal("unknown languages fall back to Portuguese")
	}
}
