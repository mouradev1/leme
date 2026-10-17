package stream

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"leme/internal/resources"
)

func row(uid, name, rv, phase string) string {
	return fmt.Sprintf(`{"cells":[%q,%q],"object":{"kind":"PartialObjectMetadata","apiVersion":"meta.k8s.io/v1",
		"metadata":{"name":%q,"namespace":"default","uid":%q,"resourceVersion":%q,"creationTimestamp":"2026-01-01T00:00:00Z",
		"labels":{"app":"web"},"ownerReferences":[{"apiVersion":"apps/v1","kind":"ReplicaSet","name":"web-123","uid":"x","controller":true}]}}}`,
		name, phase, name, uid, rv)
}

func table(cont string, rows ...string) string {
	return fmt.Sprintf(`{"kind":"Table","apiVersion":"meta.k8s.io/v1","metadata":{"resourceVersion":"10","continue":%q},
		"columnDefinitions":[{"name":"Name","type":"string","format":"name"},{"name":"Status","type":"string"}],
		"rows":[%s]}`, cont, strings.Join(rows, ","))
}

// Fake API server: 2-page list, then a watch with 3 events (2 for the same
// UID, to check coalescing), then a 410 to force a relist.
func TestWatchTable(t *testing.T) {
	var mu sync.Mutex
	watches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept"), "as=Table") || r.URL.Query().Get("includeObject") != "Metadata" {
			t.Errorf("missing Table negotiation: %v %v", r.Header, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query()
		switch {
		case q.Get("watch") == "true":
			mu.Lock()
			watches++
			n := watches
			mu.Unlock()
			if q.Get("resourceVersion") != "10" {
				t.Errorf("watch rv = %q, want 10", q.Get("resourceVersion"))
			}
			if n == 1 {
				fmt.Fprintf(w, `{"type":"MODIFIED","object":%s}`+"\n", table("", row("u1", "a", "11", "Pending")))
				fmt.Fprintf(w, `{"type":"MODIFIED","object":%s}`+"\n", table("", row("u1", "a", "12", "Running")))
				fmt.Fprintf(w, `{"type":"DELETED","object":%s}`+"\n", table("", row("u2", "b", "13", "Running")))
				w.(http.Flusher).Flush()
				time.Sleep(300 * time.Millisecond) // let the batcher flush
				fmt.Fprint(w, `{"type":"ERROR","object":{"kind":"Status","apiVersion":"v1","code":410,"reason":"Expired"}}`+"\n")
				return
			}
			<-r.Context().Done()
		case q.Get("continue") == "":
			fmt.Fprint(w, table("page2", row("u1", "a", "5", "Pending")))
		default:
			fmt.Fprint(w, table("", row("u2", "b", "6", "Running")))
		}
	}))
	defer srv.Close()

	kube := kubernetes.NewForConfigOrDie(&rest.Config{Host: srv.URL})
	ref := resources.Ref{Version: "v1", Resource: "pods", Namespaced: true}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var got []ResourceEvent
	var gmu sync.Mutex
	done := make(chan struct{})
	go func() {
		WatchTable(ctx, kube.CoreV1().RESTClient(), ref, resources.Query{Namespace: "default"}, func(ev ResourceEvent) {
			gmu.Lock()
			got = append(got, ev)
			resets := 0
			for _, e := range got {
				if e.Type == "reset" {
					resets++
				}
			}
			gmu.Unlock()
			if resets == 2 { // relisted after the 410: done
				cancel()
			}
		})
		close(done)
	}()
	<-done

	gmu.Lock()
	defer gmu.Unlock()
	var types []string
	for _, e := range got {
		types = append(types, e.Type)
	}
	want := "reset,append,synced,delta,reset"
	if strings.Join(types, ",")[:len(want)] != want {
		t.Fatalf("event sequence = %v, want prefix %s", types, want)
	}
	r0 := got[0].Rows[0]
	if r0.Labels["app"] != "web" || r0.Owner == nil || r0.Owner.Kind != "ReplicaSet" {
		t.Fatalf("labels/owner not extracted: %+v", r0)
	}
	if len(got[0].Columns) != 2 || r0.UID != "u1" || r0.Created.IsZero() {
		t.Fatalf("bad reset payload: %+v", got[0])
	}
	d := got[3].Deltas
	if len(d) != 2 || d[0].Row.Cells[1] != "Running" || d[1].Type != "DELETED" {
		t.Fatalf("deltas not coalesced per UID: %+v", d)
	}
}
