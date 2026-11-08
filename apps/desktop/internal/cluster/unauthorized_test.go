package cluster

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUnauthorizedNotifier(t *testing.T) {
	code := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
	defer srv.Close()
	got := make(chan struct{}, 2)
	c := &http.Client{Transport: unauthorizedNotifier{rt: http.DefaultTransport, fn: func() { got <- struct{}{} }}}

	resp, _ := c.Get(srv.URL)
	resp.Body.Close()
	select {
	case <-got:
		t.Fatal("200 must not notify")
	case <-time.After(50 * time.Millisecond):
	}
	code = http.StatusUnauthorized
	resp, _ = c.Get(srv.URL)
	resp.Body.Close()
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("401 not reported")
	}
}
