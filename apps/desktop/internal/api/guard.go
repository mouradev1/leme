package api

import (
	"errors"
	"fmt"
	"leme/internal/platform/i18n"

	"leme/internal/platform/settings"
)

// ClusterPrefs returns the safety marks of every cluster that has one.
func (a *ClusterAPI) ClusterPrefs() map[string]settings.ClusterPrefs {
	p := settings.Load().Clusters
	if p == nil {
		p = map[string]settings.ClusterPrefs{}
	}
	return p
}

// SetClusterPrefs marks a cluster as production and/or read-only.
func (a *ClusterAPI) SetClusterPrefs(name string, prefs settings.ClusterPrefs) error {
	if name == "" {
		return errors.New(i18n.T(i18n.T("cluster sem nome", "cluster without a name"), "cluster without a name"))
	}
	s := settings.Load()
	if s.Clusters == nil {
		s.Clusters = map[string]settings.ClusterPrefs{}
	}
	if prefs == (settings.ClusterPrefs{}) {
		delete(s.Clusters, name)
	} else {
		s.Clusters[name] = prefs
	}
	return settings.Save(s)
}

// writable refuses changes to a read-only cluster. Every API call that
// changes a cluster (or opens a shell into it) checks this first, so the
// mark holds even if the UI were bypassed.
func (d *Deps) writable(cluster string) error {
	if settings.Load().Clusters[cluster].ReadOnly {
		return fmt.Errorf(i18n.T("o cluster %s está em modo somente leitura (desmarque no clique direito do cluster para alterar algo)", "cluster %s is read-only (turn it off in the cluster's right-click menu to change anything)"), cluster)
	}
	return nil
}
