package stream

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestWriteLogsTagsSourcesWhenSeveral(t *testing.T) {
	kube := fake.NewSimpleClientset() // the fake returns "fake logs" for any pod
	var buf bytes.Buffer
	o := LogOptions{Namespace: "ns", Targets: []LogTarget{{Pod: "a", Container: "app"}, {Pod: "b", Container: "app"}}}
	n, err := WriteLogs(context.Background(), kube, o, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("lines = %d, want 2", n)
	}
	want := "[a/app] fake logs\n[b/app] fake logs\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}

	buf.Reset()
	o.Targets = o.Targets[:1]
	if _, err := WriteLogs(context.Background(), kube, o, &buf); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "[") {
		t.Fatalf("single source must not be tagged: %q", buf.String())
	}
}

func TestCreatePrivateIsOwnerOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.log")
	if err := os.WriteFile(p, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := CreatePrivate(p)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 || st.Size() != 0 {
		t.Fatalf("mode=%v size=%d, want 0600 and truncated", st.Mode().Perm(), st.Size())
	}
}
