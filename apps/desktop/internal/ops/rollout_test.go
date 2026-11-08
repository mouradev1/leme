package ops

import (
	"context"
	"strconv"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"leme/internal/cluster"
)

func tpl(image, hash string) corev1.PodTemplateSpec {
	l := map[string]string{"app": "api"}
	if hash != "" {
		l[appsv1.DefaultDeploymentUniqueLabelKey] = hash
	}
	return corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: l}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "api", Image: image}}}}
}

func rs(rev int, image string, owner types.UID) *appsv1.ReplicaSet {
	yes := true
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "api-" + strconv.Itoa(rev), Namespace: "ns", Labels: map[string]string{"app": "api"},
			Annotations:     map[string]string{revisionAnno: strconv.Itoa(rev), changeCauseAnno: "deploy " + image},
			OwnerReferences: []metav1.OwnerReference{{UID: owner, Controller: &yes, Kind: "Deployment", Name: "api"}},
		},
		Spec: appsv1.ReplicaSetSpec{Template: tpl(image, "h"+strconv.Itoa(rev))},
	}
}

func TestRolloutStatusAndUndo(t *testing.T) {
	three := int32(3)
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "ns", UID: "d1", Generation: 4, Annotations: map[string]string{revisionAnno: "3"}},
		Spec:       appsv1.DeploymentSpec{Replicas: &three, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}}, Template: tpl("api:3", "")},
		Status:     appsv1.DeploymentStatus{ObservedGeneration: 4, Replicas: 4, UpdatedReplicas: 3, ReadyReplicas: 3, AvailableReplicas: 3},
	}
	other := rs(9, "other:1", "someone-else")
	s := &cluster.Session{Kube: fake.NewSimpleClientset(d, rs(1, "api:1", "d1"), rs(2, "api:2", "d1"), rs(3, "api:3", "d1"), other)}
	ctx := context.Background()

	r, err := RolloutStatus(ctx, s, "ns", "api")
	if err != nil {
		t.Fatal(err)
	}
	if r.Done || r.Message != "encerrando 1 pod(s) da versão antiga" {
		t.Fatalf("status: %+v", r)
	}
	if len(r.History) != 3 || r.History[0].Revision != 3 || !r.History[0].Current || r.History[2].Images[0] != "api:1" {
		t.Fatalf("history: %+v", r.History)
	}

	rev, err := RolloutUndo(ctx, s, "ns", "api", 0) // previous = 2
	if err != nil || rev != 2 {
		t.Fatalf("undo: %d %v", rev, err)
	}
	got, _ := s.Kube.AppsV1().Deployments("ns").Get(ctx, "api", metav1.GetOptions{})
	if got.Spec.Template.Spec.Containers[0].Image != "api:2" {
		t.Fatal("template not restored")
	}
	if _, ok := got.Spec.Template.Labels[appsv1.DefaultDeploymentUniqueLabelKey]; ok {
		t.Fatal("pod-template-hash must not be copied")
	}
	if got.Annotations[changeCauseAnno] != "deploy api:2" {
		t.Fatal("change-cause not carried over")
	}
	if _, err := RolloutUndo(ctx, s, "ns", "api", 7); err == nil {
		t.Fatal("missing revision accepted")
	}
	if _, err := RolloutUndo(ctx, s, "ns", "api", 3); err == nil {
		t.Fatal("undo to the current revision accepted")
	}
}
