package resources

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"leme/internal/cluster"
)

func TestSecretDataRoundTrip(t *testing.T) {
	bin := []byte{0xff, 0xfe, 0x00, 0x01}
	kube := fake.NewClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns", ResourceVersion: "1"},
		Data:       map[string][]byte{"user": []byte("admin"), "old": []byte("x"), "keystore": bin},
	})
	s := &cluster.Session{Kube: kube}
	ctx := context.Background()

	d, err := GetData(ctx, s, "secrets", "ns", "s")
	if err != nil {
		t.Fatal(err)
	}
	if d.Data["user"] != "admin" || len(d.Binary) != 1 || d.Binary[0] != "keystore" {
		t.Fatalf("GetData = %+v", d)
	}

	// edit "user", drop "old", add "pass"
	if _, err := SetData(ctx, s, "secrets", "ns", "s", map[string]string{"user": "root", "pass": "p@ss"}, d.ResourceVersion); err != nil {
		t.Fatal(err)
	}
	got, _ := kube.CoreV1().Secrets("ns").Get(ctx, "s", metav1.GetOptions{})
	if string(got.Data["user"]) != "root" || string(got.Data["pass"]) != "p@ss" {
		t.Fatalf("text keys not saved: %v", got.Data)
	}
	if _, ok := got.Data["old"]; ok {
		t.Fatal("removed key still present")
	}
	if string(got.Data["keystore"]) != string(bin) {
		t.Fatal("binary key was not preserved")
	}
}
