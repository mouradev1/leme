package resources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/client-go/rest"

	"leme/internal/cluster"
)

func TestSearchRanksAndSkipsForbidden(t *testing.T) {
	names := map[string][]string{
		"/api/v1/pods":              {"api-7d9f-abcde", "web-api-1", "db-0"},
		"/apis/apps/v1/deployments": {"api", "billing"},
		"/api/v1/secrets":           nil, // 403
		"/api/v1/nodes":             {"node-api-pool-1"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept"), "PartialObjectMetadata") {
			t.Errorf("must ask for metadata only, got Accept %q", r.Header.Get("Accept"))
		}
		list, ok := names[r.URL.Path]
		if r.URL.Path == "/api/v1/secrets" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var items []string
		for _, n := range list {
			items = append(items, fmt.Sprintf(`{"metadata":{"name":%q,"namespace":"ns"}}`, n))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"kind":"PartialObjectMetadataList","apiVersion":"meta.k8s.io/v1","metadata":{},"items":[%s]}`, strings.Join(items, ","))
	}))
	defer srv.Close()
	s := &cluster.Session{Config: &rest.Config{Host: srv.URL}}

	hits, err := Search(context.Background(), s, nil, "API", 10)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, h := range hits {
		got = append(got, h.Kind+"/"+h.Name)
	}
	want := "Deployment/api,Pod/api-7d9f-abcde,Pod/web-api-1,Node/node-api-pool-1"
	if strings.Join(got, ",") != want {
		t.Fatalf("got %s\nwant %s", strings.Join(got, ","), want)
	}
	if hits, _ := Search(context.Background(), s, nil, "a", 10); len(hits) != 0 {
		t.Fatal("one-letter queries must not list anything")
	}
}
