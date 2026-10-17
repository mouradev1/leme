package api

import (
	"context"
	"time"

	"leme/internal/helm"
)

type HelmAPI struct{ d *Deps }

func (a *HelmAPI) client(cluster string) (*helm.Client, error) {
	s, err := a.d.session(cluster)
	if err != nil {
		return nil, err
	}
	return helm.New(s), nil
}

func (a *HelmAPI) ListReleases(cluster, namespace string) ([]helm.Release, error) {
	c, err := a.client(cluster)
	if err != nil {
		return nil, err
	}
	return c.List(namespace)
}

func (a *HelmAPI) History(cluster, namespace, name string) ([]helm.Release, error) {
	c, err := a.client(cluster)
	if err != nil {
		return nil, err
	}
	return c.History(namespace, name)
}

func (a *HelmAPI) Values(cluster, namespace, name string, all bool) (string, error) {
	c, err := a.client(cluster)
	if err != nil {
		return "", err
	}
	return c.Values(namespace, name, all)
}

func (a *HelmAPI) Rollback(cluster, namespace, name string, revision int) error {
	c, err := a.client(cluster)
	if err != nil {
		return err
	}
	return c.Rollback(namespace, name, revision)
}

func (a *HelmAPI) Uninstall(cluster, namespace, name string) error {
	c, err := a.client(cluster)
	if err != nil {
		return err
	}
	return c.Uninstall(namespace, name)
}

func (a *HelmAPI) Install(cluster string, req helm.InstallRequest) (helm.Release, error) {
	c, err := a.client(cluster)
	if err != nil {
		return helm.Release{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return c.Install(ctx, req)
}
