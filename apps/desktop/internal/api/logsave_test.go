package api

import "testing"

func TestSafeFileName(t *testing.T) {
	cases := map[string]string{
		"api-7d9f_2026-11-06.log": "api-7d9f_2026-11-06.log",
		"../../etc/passwd":        "passwd.log",
		"a b/c:d":                 "c-d.log",
		"":                        "logs.log",
		"...":                     "logs.log",
		"x.txt":                   "x.txt",
	}
	for in, want := range cases {
		if got := safeFileName(in); got != want {
			t.Errorf("safeFileName(%q) = %q, want %q", in, got, want)
		}
	}
}
