package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.2.0", "0.1.1", true},
		{"v0.2.0", "0.2.0", false},
		{"0.1.10", "0.1.9", true}, // numeric, not lexical
		{"0.2.0-rc.1", "0.2.0", false},
		{"0.2.0", "0.2.0-rc.1", true},
		{"0.2.0", "dev", true},
		{"garbage", "0.1.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestLatest(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		if r.URL.Path != "/repos/"+Repo+"/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://github.com/mouradev1/leme/releases/tag/v0.2.0","body":"## Install","published_at":"2026-10-17T18:00:00Z"}`))
	}))
	defer srv.Close()
	c := &Checker{BaseURL: srv.URL, Client: srv.Client()}

	info, err := c.Latest(context.Background(), "0.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "0.2.0" || info.Current != "0.1.1" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if ua != "Leme" {
		t.Fatalf("user agent must not leak version/OS: %q", ua)
	}
	if info, _ := c.Latest(context.Background(), "0.2.0"); info.Available {
		t.Fatal("same version must not be reported as an update")
	}
}

func TestLatestRejectsForeignURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.0.0","html_url":"https://evil.example/download"}`))
	}))
	defer srv.Close()
	info, err := (&Checker{BaseURL: srv.URL, Client: srv.Client()}).Latest(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if info.URL != "https://github.com/"+Repo+"/releases/latest" {
		t.Fatalf("download link must stay on the project's GitHub: %s", info.URL)
	}
}
