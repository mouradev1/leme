package api

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"leme/internal/portforward"
)

type PortForwardAPI struct{ d *Deps }

func (a *PortForwardAPI) List() []portforward.Forward { return a.d.Forwards.List() }

// Start opens 127.0.0.1:<local> -> pod/service port. Updates arrive on
// "portforwards:changed".
func (a *PortForwardAPI) Start(cluster string, req portforward.Request) (portforward.Forward, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return portforward.Forward{}, err
	}
	return a.d.Forwards.Start(s, req)
}

func (a *PortForwardAPI) Stop(id string) { a.d.Forwards.Stop(id) }

type ServicePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort string `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

// ServicePorts lists a Service's ports for the forward dialog.
func (a *PortForwardAPI) ServicePorts(cluster, namespace, name string) ([]ServicePort, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(s.Ctx(), 15*time.Second)
	defer cancel()
	svc, err := s.Kube.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ServicePort, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		out = append(out, ServicePort{Name: p.Name, Port: p.Port, TargetPort: p.TargetPort.String(), Protocol: string(p.Protocol)})
	}
	return out, nil
}
