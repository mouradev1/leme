package pods

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pod(status corev1.ContainerStatus) *corev1.Pod {
	return &corev1.Pod{
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "app",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			}},
		}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{status}},
	}
}

func ev(reason, msg string, ago time.Duration) corev1.Event {
	return corev1.Event{
		Reason: reason, Message: msg, LastTimestamp: metav1.NewTime(time.Now().Add(-ago)),
		InvolvedObject: corev1.ObjectReference{FieldPath: "spec.containers{app}"},
	}
}

func term(reason string, code int32) *corev1.ContainerStateTerminated {
	return &corev1.ContainerStateTerminated{Reason: reason, ExitCode: code, FinishedAt: metav1.Now()}
}

func only(t *testing.T, ds []Diagnosis) Diagnosis {
	t.Helper()
	if len(ds) != 1 {
		t.Fatalf("want 1 diagnosis, got %d: %+v", len(ds), ds)
	}
	return ds[0]
}

func TestOOMKilled(t *testing.T) {
	d := only(t, Diagnose(pod(corev1.ContainerStatus{
		Name: "app", RestartCount: 3,
		State:                corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		LastTerminationState: corev1.ContainerState{Terminated: term("OOMKilled", 137)},
	}), nil))
	if !strings.Contains(d.Title, "OOMKilled") || !strings.Contains(d.Detail, "512Mi") || d.Severity != "warn" {
		t.Fatalf("%+v", d)
	}
}

func TestLivenessKill(t *testing.T) {
	d := only(t, Diagnose(pod(corev1.ContainerStatus{
		Name: "app", RestartCount: 2,
		State:                corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		LastTerminationState: corev1.ContainerState{Terminated: term("Error", 137)},
	}), []corev1.Event{
		ev("Unhealthy", "Liveness probe failed: HTTP probe failed with statuscode: 503", time.Minute),
		ev("Killing", "Container app failed liveness probe, will be restarted", time.Minute),
	}))
	if !strings.Contains(d.Title, "probe") || !strings.Contains(d.Detail, "503") {
		t.Fatalf("liveness kill not identified: %+v", d)
	}
}

func TestCrashLoopAppError(t *testing.T) {
	d := only(t, Diagnose(pod(corev1.ContainerStatus{
		Name: "app", RestartCount: 41,
		State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		LastTerminationState: corev1.ContainerState{Terminated: term("Error", 1)},
	}), nil))
	if d.Severity != "error" || !strings.Contains(d.Title, "CrashLoopBackOff") || !strings.Contains(d.Title, "exit 1") {
		t.Fatalf("%+v", d)
	}
}

func TestUnschedulable(t *testing.T) {
	p := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Message: "0/3 nodes are available: 3 Insufficient cpu.",
	}}}}
	d := only(t, Diagnose(p, nil))
	if !strings.Contains(d.Detail, "Insufficient cpu") {
		t.Fatalf("%+v", d)
	}
}

func TestHealthyPodHasNoDiagnosis(t *testing.T) {
	ds := Diagnose(pod(corev1.ContainerStatus{Name: "app", Ready: true,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}), nil)
	if len(ds) != 0 {
		t.Fatalf("healthy pod should have no diagnoses: %+v", ds)
	}
}
