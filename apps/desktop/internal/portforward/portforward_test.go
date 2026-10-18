package portforward

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func pod(name string, ready bool, ports ...corev1.ContainerPort) corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Ports: ports}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}},
	}
}

func svc(ports ...corev1.ServicePort) *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web"}, Spec: corev1.ServiceSpec{Ports: ports}}
}

func TestServiceTarget(t *testing.T) {
	notReady := pod("web-old", false, corev1.ContainerPort{Name: "http", ContainerPort: 8080})
	terminating := pod("web-going", true, corev1.ContainerPort{Name: "http", ContainerPort: 8080})
	now := metav1.Now()
	terminating.DeletionTimestamp = &now
	good := pod("web-ok", true, corev1.ContainerPort{Name: "http", ContainerPort: 8080})
	pods := []corev1.Pod{notReady, terminating, good}

	cases := []struct {
		name     string
		svc      *corev1.Service
		port     int32
		wantPod  string
		wantPort int32
	}{
		{"numeric targetPort", svc(corev1.ServicePort{Port: 80, TargetPort: intstr.FromInt32(8080)}), 80, "web-ok", 8080},
		{"named targetPort", svc(corev1.ServicePort{Port: 80, TargetPort: intstr.FromString("http")}), 80, "web-ok", 8080},
		{"no targetPort = same port", svc(corev1.ServicePort{Port: 9090}), 9090, "web-ok", 9090},
	}
	for _, c := range cases {
		gotPod, gotPort, err := ServiceTarget(c.svc, pods, c.port)
		if err != nil || gotPod != c.wantPod || gotPort != c.wantPort {
			t.Errorf("%s: got (%s, %d, %v), want (%s, %d)", c.name, gotPod, gotPort, err, c.wantPod, c.wantPort)
		}
	}

	if _, _, err := ServiceTarget(svc(corev1.ServicePort{Port: 80}), pods, 443); err == nil || !strings.Contains(err.Error(), "não expõe a porta 443") {
		t.Errorf("unknown service port: %v", err)
	}
	if _, _, err := ServiceTarget(svc(corev1.ServicePort{Port: 80}), []corev1.Pod{notReady, terminating}, 80); err == nil || !strings.Contains(err.Error(), "nenhum pod pronto") {
		t.Errorf("no ready pods: %v", err)
	}
}

func TestFriendlyPortInUse(t *testing.T) {
	err := friendly(errString("unable to listen on any of the requested ports: [{8080 80}]"), 8080)
	if !strings.Contains(err.Error(), "porta local 8080 já está em uso") {
		t.Fatalf("got %v", err)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
