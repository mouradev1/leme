package api

import (
	"context"
	"fmt"
	"leme/internal/platform/i18n"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"leme/internal/rancher"
)

// RancherAPI: add Rancher servers, log in through the browser, list their
// clusters. The clusters then appear in the normal cluster list.
type RancherAPI struct{ d *Deps }

func (a *RancherAPI) List() []rancher.Status { return a.d.Rancher.List() }

func (a *RancherAPI) Add(url, name string) (rancher.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return a.d.Rancher.Add(ctx, url, name)
}

// Login opens Rancher's login page in the default browser and returns once
// the user has logged in there (or after 5 minutes / CancelLogin).
func (a *RancherAPI) Login(url string) (rancher.Status, error) {
	return a.d.Rancher.Login(context.Background(), url, func(u string) {
		runtime.BrowserOpenURL(a.d.Bus.Context(), u)
	})
}

func (a *RancherAPI) CancelLogin(url string) { a.d.Rancher.CancelLogin(url) }

// NamespaceProjects maps each namespace of a Rancher cluster to its Rancher
// Project name (namespaces carry the project id in a label). Empty for other
// clusters or without permission.
func (a *RancherAPI) NamespaceProjects(cluster string) (map[string]string, error) {
	out := map[string]string{}
	s, err := a.d.session(cluster)
	if err != nil {
		return out, err
	}
	ctx, cancel := reqCtx(s)
	defer cancel()
	names, err := a.d.Rancher.ProjectNames(ctx, cluster)
	if err != nil || len(names) == 0 {
		return out, nil
	}
	list, err := s.Kube.CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: projectLabel})
	if err != nil {
		return out, nil
	}
	for _, ns := range list.Items {
		id := ns.Labels[projectLabel]
		if n := names[id]; n != "" {
			out[ns.Name] = n
		}
	}
	return out, nil
}

const projectLabel = "field.cattle.io/projectId"

// Info: server version and enabled login methods (to explain login issues).
func (a *RancherAPI) Info(url string) (rancher.Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.d.Rancher.Info(ctx, url)
}

func (a *RancherAPI) LoginWithKey(url, key string) (rancher.Status, error) {
	return a.d.Rancher.LoginWithKey(context.Background(), url, key)
}

func (a *RancherAPI) Refresh(url string) (rancher.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return a.d.Rancher.Refresh(ctx, url)
}

func (a *RancherAPI) Logout(url string) error {
	a.stopClusters(url)
	return a.d.Rancher.Logout(context.Background(), url)
}

func (a *RancherAPI) Remove(url string) error {
	a.stopClusters(url)
	return a.d.Rancher.Remove(context.Background(), url)
}

// stopClusters closes the server's open connections and port-forwards.
func (a *RancherAPI) stopClusters(url string) {
	for _, s := range a.d.Rancher.List() {
		if s.URL != url {
			continue
		}
		for _, c := range s.Clusters {
			if a.d.Forwards != nil {
				a.d.Forwards.StopCluster(c)
			}
			a.d.Clusters.Disconnect(c)
		}
	}
}

// RancherLoginNeeded is the error text prefix the UI recognizes to offer a
// one-click login: "rancher-login:<url>|<message>".
const RancherLoginNeeded = "rancher-login:"

func loginNeeded(url, msg string) error {
	return fmt.Errorf("%s%s|%s", RancherLoginNeeded, url, msg)
}

// rancherReady fails fast (no network) for a Rancher cluster while logged out.
func (d *Deps) rancherReady(context string) error {
	if d.Rancher == nil {
		return nil
	}
	u, name, ok := d.Rancher.Owner(context)
	if !ok || d.Rancher.HasToken(u) {
		return nil
	}
	return loginNeeded(u, fmt.Sprintf(i18n.T("Entre no Rancher %s para acessar este cluster.", "Log in to Rancher %s to access this cluster."), name))
}

// rancherErr turns a 401 from a Rancher cluster into "log in again".
func (d *Deps) rancherErr(context string, err error) error {
	if err == nil || d.Rancher == nil || !apierrors.IsUnauthorized(err) {
		return err
	}
	u, name, ok := d.Rancher.Owner(context)
	if !ok {
		return err
	}
	d.Rancher.MarkExpired(u)
	return loginNeeded(u, fmt.Sprintf(i18n.T("A sessão do Rancher %s expirou. Entre de novo.", "The Rancher %s session expired. Log in again."), name))
}
