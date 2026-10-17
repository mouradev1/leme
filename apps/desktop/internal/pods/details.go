// Package pods builds the Pod details panel: a flattened, UI-friendly view of
// spec+status plus a plain-language diagnosis of why containers restarted or
// aren't running (see diagnose.go).
package pods

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Details struct {
	Phase          string            `json:"phase"`
	Reason         string            `json:"reason"`
	Message        string            `json:"message"`
	Node           string            `json:"node"`
	PodIP          string            `json:"podIP"`
	PodIPs         []string          `json:"podIPs"`
	HostIP         string            `json:"hostIP"`
	ServiceAccount string            `json:"serviceAccount"`
	QOSClass       string            `json:"qosClass"`
	PriorityClass  string            `json:"priorityClass"`
	RestartPolicy  string            `json:"restartPolicy"`
	StartTime      string            `json:"startTime"`
	Conditions     []Condition       `json:"conditions"`
	Tolerations    []string          `json:"tolerations"`
	NodeSelector   map[string]string `json:"nodeSelector"`
	Volumes        []Volume          `json:"volumes"`
	InitContainers []Container       `json:"initContainers"`
	Containers     []Container       `json:"containers"`
	Diagnoses      []Diagnosis       `json:"diagnoses"`
}

type Condition struct {
	Type           string `json:"type"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	LastTransition string `json:"lastTransition"`
}

type Volume struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`   // ConfigMap, Secret, PVC, EmptyDir...
	Source string `json:"source"` // referenced object/path, if any
}

type Container struct {
	Name            string       `json:"name"`
	Init            bool         `json:"init"`
	Image           string       `json:"image"`
	ImagePullPolicy string       `json:"imagePullPolicy"`
	State           string       `json:"state"` // running | waiting | terminated | unknown
	StateReason     string       `json:"stateReason"`
	StateMessage    string       `json:"stateMessage"`
	StartedAt       string       `json:"startedAt"`
	Ready           bool         `json:"ready"`
	RestartCount    int32        `json:"restartCount"`
	LastTermination *Termination `json:"lastTermination"`
	Ports           []Port       `json:"ports"`
	Env             []EnvVar     `json:"env"`
	EnvFrom         []string     `json:"envFrom"`
	Mounts          []Mount      `json:"mounts"`
	Liveness        *Probe       `json:"liveness"`
	Readiness       *Probe       `json:"readiness"`
	Startup         *Probe       `json:"startup"`
	Requests        Resources    `json:"requests"`
	Limits          Resources    `json:"limits"`
}

type Termination struct {
	Reason     string `json:"reason"`
	ExitCode   int32  `json:"exitCode"`
	Signal     int32  `json:"signal"`
	Message    string `json:"message"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	Meaning    string `json:"meaning"` // human explanation of the exit code
}

type Port struct {
	Name          string `json:"name"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	From  string `json:"from"` // "secret db/password", "configMap app/url", "field metadata.name"
}

type Mount struct {
	Path     string `json:"path"`
	Volume   string `json:"volume"`
	ReadOnly bool   `json:"readOnly"`
	SubPath  string `json:"subPath"`
}

type Probe struct {
	Handler string `json:"handler"` // http-get | tcp | exec | grpc
	Target  string `json:"target"`
	Delay   int32  `json:"delay"`
	Timeout int32  `json:"timeout"`
	Period  int32  `json:"period"`
	Success int32  `json:"success"`
	Failure int32  `json:"failure"`
}

// Resources: 0 means "not set".
type Resources struct {
	CPUMilli    int64 `json:"cpu"`
	MemoryBytes int64 `json:"memory"`
}

// Get loads the pod and its events and builds Details. Secret values are never
// resolved: env vars from secrets only show the reference.
func Get(ctx context.Context, kube kubernetes.Interface, ns, name string) (Details, error) {
	p, err := kube.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return Details{}, err
	}
	evs, err := kube.CoreV1().Events(ns).List(ctx, metav1.ListOptions{FieldSelector: "involvedObject.uid=" + string(p.UID)})
	var events []corev1.Event
	if err == nil {
		events = evs.Items
	}
	return Build(p, events), nil
}

// Build is pure (no I/O) so it's unit-testable.
func Build(p *corev1.Pod, events []corev1.Event) Details {
	d := Details{
		Phase: string(p.Status.Phase), Reason: p.Status.Reason, Message: p.Status.Message,
		Node: p.Spec.NodeName, PodIP: p.Status.PodIP, HostIP: p.Status.HostIP,
		ServiceAccount: p.Spec.ServiceAccountName, QOSClass: string(p.Status.QOSClass),
		PriorityClass: p.Spec.PriorityClassName, RestartPolicy: string(p.Spec.RestartPolicy),
		NodeSelector: p.Spec.NodeSelector,
	}
	if p.Status.StartTime != nil {
		d.StartTime = ts(p.Status.StartTime.Time)
	}
	for _, ip := range p.Status.PodIPs {
		d.PodIPs = append(d.PodIPs, ip.IP)
	}
	for _, c := range p.Status.Conditions {
		d.Conditions = append(d.Conditions, Condition{
			Type: string(c.Type), Status: string(c.Status), Reason: c.Reason, Message: c.Message,
			LastTransition: ts(c.LastTransitionTime.Time),
		})
	}
	for _, t := range p.Spec.Tolerations {
		d.Tolerations = append(d.Tolerations, toleration(t))
	}
	for _, v := range p.Spec.Volumes {
		d.Volumes = append(d.Volumes, volume(v))
	}

	statuses := map[string]corev1.ContainerStatus{}
	for _, s := range append(append([]corev1.ContainerStatus{}, p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...) {
		statuses[s.Name] = s
	}
	for _, c := range p.Spec.InitContainers {
		d.InitContainers = append(d.InitContainers, container(c, statuses[c.Name], true))
	}
	for _, c := range p.Spec.Containers {
		d.Containers = append(d.Containers, container(c, statuses[c.Name], false))
	}
	d.Diagnoses = Diagnose(p, events)
	return d
}

func container(c corev1.Container, s corev1.ContainerStatus, init bool) Container {
	out := Container{
		Name: c.Name, Init: init, Image: c.Image, ImagePullPolicy: string(c.ImagePullPolicy),
		Ready: s.Ready, RestartCount: s.RestartCount, State: "unknown",
		Liveness: probe(c.LivenessProbe), Readiness: probe(c.ReadinessProbe), Startup: probe(c.StartupProbe),
		Requests: resources(c.Resources.Requests), Limits: resources(c.Resources.Limits),
	}
	switch {
	case s.State.Running != nil:
		out.State, out.StartedAt = "running", ts(s.State.Running.StartedAt.Time)
	case s.State.Waiting != nil:
		out.State, out.StateReason, out.StateMessage = "waiting", s.State.Waiting.Reason, s.State.Waiting.Message
	case s.State.Terminated != nil:
		t := s.State.Terminated
		out.State, out.StateReason, out.StateMessage = "terminated", t.Reason, t.Message
	}
	if t := s.LastTerminationState.Terminated; t != nil {
		out.LastTermination = &Termination{
			Reason: t.Reason, ExitCode: t.ExitCode, Signal: t.Signal, Message: strings.TrimSpace(t.Message),
			StartedAt: ts(t.StartedAt.Time), FinishedAt: ts(t.FinishedAt.Time), Meaning: exitMeaning(t.Reason, t.ExitCode),
		}
	}
	for _, p := range c.Ports {
		out.Ports = append(out.Ports, Port{Name: p.Name, ContainerPort: p.ContainerPort, Protocol: string(p.Protocol)})
	}
	for _, e := range c.Env {
		out.Env = append(out.Env, envVar(e))
	}
	for _, e := range c.EnvFrom {
		switch {
		case e.ConfigMapRef != nil:
			out.EnvFrom = append(out.EnvFrom, "configMap "+e.ConfigMapRef.Name)
		case e.SecretRef != nil:
			out.EnvFrom = append(out.EnvFrom, "secret "+e.SecretRef.Name)
		}
	}
	for _, m := range c.VolumeMounts {
		out.Mounts = append(out.Mounts, Mount{Path: m.MountPath, Volume: m.Name, ReadOnly: m.ReadOnly, SubPath: m.SubPath})
	}
	return out
}

func envVar(e corev1.EnvVar) EnvVar {
	v := EnvVar{Name: e.Name, Value: e.Value}
	if f := e.ValueFrom; f != nil {
		switch {
		case f.SecretKeyRef != nil:
			v.From = fmt.Sprintf("secret %s/%s", f.SecretKeyRef.Name, f.SecretKeyRef.Key)
		case f.ConfigMapKeyRef != nil:
			v.From = fmt.Sprintf("configMap %s/%s", f.ConfigMapKeyRef.Name, f.ConfigMapKeyRef.Key)
		case f.FieldRef != nil:
			v.From = "field " + f.FieldRef.FieldPath
		case f.ResourceFieldRef != nil:
			v.From = "resource " + f.ResourceFieldRef.Resource
		}
	}
	return v
}

func probe(p *corev1.Probe) *Probe {
	if p == nil {
		return nil
	}
	out := &Probe{Delay: p.InitialDelaySeconds, Timeout: p.TimeoutSeconds, Period: p.PeriodSeconds,
		Success: p.SuccessThreshold, Failure: p.FailureThreshold}
	switch {
	case p.HTTPGet != nil:
		scheme := strings.ToLower(string(p.HTTPGet.Scheme))
		if scheme == "" {
			scheme = "http"
		}
		out.Handler, out.Target = "http-get", fmt.Sprintf("%s://:%s%s", scheme, p.HTTPGet.Port.String(), p.HTTPGet.Path)
	case p.TCPSocket != nil:
		out.Handler, out.Target = "tcp", ":"+p.TCPSocket.Port.String()
	case p.Exec != nil:
		out.Handler, out.Target = "exec", strings.Join(p.Exec.Command, " ")
	case p.GRPC != nil:
		out.Handler, out.Target = "grpc", fmt.Sprintf(":%d", p.GRPC.Port)
	}
	return out
}

func resources(l corev1.ResourceList) Resources {
	var r Resources
	if q, ok := l[corev1.ResourceCPU]; ok {
		r.CPUMilli = q.MilliValue()
	}
	if q, ok := l[corev1.ResourceMemory]; ok {
		r.MemoryBytes = q.Value()
	}
	return r
}

func volume(v corev1.Volume) Volume {
	s := v.VolumeSource
	switch {
	case s.ConfigMap != nil:
		return Volume{v.Name, "ConfigMap", s.ConfigMap.Name}
	case s.Secret != nil:
		return Volume{v.Name, "Secret", s.Secret.SecretName}
	case s.PersistentVolumeClaim != nil:
		return Volume{v.Name, "PersistentVolumeClaim", s.PersistentVolumeClaim.ClaimName}
	case s.EmptyDir != nil:
		return Volume{v.Name, "EmptyDir", string(s.EmptyDir.Medium)}
	case s.HostPath != nil:
		return Volume{v.Name, "HostPath", s.HostPath.Path}
	case s.Projected != nil:
		return Volume{v.Name, "Projected", fmt.Sprintf("%d sources", len(s.Projected.Sources))}
	case s.DownwardAPI != nil:
		return Volume{v.Name, "DownwardAPI", ""}
	case s.CSI != nil:
		return Volume{v.Name, "CSI", s.CSI.Driver}
	case s.Ephemeral != nil:
		return Volume{v.Name, "Ephemeral", ""}
	case s.NFS != nil:
		return Volume{v.Name, "NFS", s.NFS.Server + ":" + s.NFS.Path}
	}
	return Volume{v.Name, "Other", ""}
}

func toleration(t corev1.Toleration) string {
	key := t.Key
	if key == "" {
		key = "*"
	}
	s := key
	if t.Operator == corev1.TolerationOpEqual && t.Value != "" {
		s += "=" + t.Value
	} else if t.Operator == corev1.TolerationOpExists {
		s += " (exists)"
	}
	if t.Effect != "" {
		s += ":" + string(t.Effect)
	}
	if t.TolerationSeconds != nil {
		s += fmt.Sprintf(" for %ds", *t.TolerationSeconds)
	}
	return s
}

func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// sortEvents newest first (by last occurrence).
func sortEvents(evs []corev1.Event) []corev1.Event {
	out := append([]corev1.Event(nil), evs...)
	last := func(e corev1.Event) time.Time {
		if !e.LastTimestamp.IsZero() {
			return e.LastTimestamp.Time
		}
		if !e.EventTime.IsZero() {
			return e.EventTime.Time
		}
		return e.CreationTimestamp.Time
	}
	sort.SliceStable(out, func(i, j int) bool { return last(out[i]).After(last(out[j])) })
	return out
}
