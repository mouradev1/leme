package cluster

import (
	"testing"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type nopEmitter struct{}

func (nopEmitter) Emit(string, any) {}

func overlayCfg(ctx, server, token string) *clientcmdapi.Config {
	c := clientcmdapi.NewConfig()
	c.Clusters["rancher:x/"+ctx] = &clientcmdapi.Cluster{Server: server}
	c.AuthInfos["rancher:x"] = &clientcmdapi.AuthInfo{Token: token}
	c.Contexts[ctx] = &clientcmdapi.Context{Cluster: "rancher:x/" + ctx, AuthInfo: "rancher:x"}
	return c
}

func TestOverlayAddsReplacesAndRemovesContexts(t *testing.T) {
	files := clientcmdapi.NewConfig()
	files.Clusters["mine"] = &clientcmdapi.Cluster{Server: "https://mine"}
	files.AuthInfos["me"] = &clientcmdapi.AuthInfo{Token: "t"}
	files.Contexts["prod/api"] = &clientcmdapi.Context{Cluster: "mine", AuthInfo: "me"}
	m := &Manager{bus: nopEmitter{}, sessions: map[string]*Session{}, overlays: map[string]*clientcmdapi.Config{}, files: files}
	m.rebuildLocked()

	m.SetOverlay("https://r", overlayCfg("prod/web", "https://r/k8s/clusters/c-1", "old"))
	if m.raw.Contexts["prod/web"] == nil {
		t.Fatal("overlay context missing")
	}

	// A file context with the same name always wins.
	m.SetOverlay("https://r", overlayCfg("prod/api", "https://r/k8s/clusters/c-2", "x"))
	if m.raw.Clusters[m.raw.Contexts["prod/api"].Cluster].Server != "https://mine" {
		t.Fatal("overlay shadowed a kubeconfig context")
	}

	// New token: an open session for that context is closed (reconnects).
	m.SetOverlay("https://r", overlayCfg("prod/web", "https://r/k8s/clusters/c-1", "old"))
	s := &Session{fingerprint: fingerprint(m.raw, "prod/web")}
	s.ctx, s.cancel = t.Context(), func() {}
	m.sessions["prod/web"] = s
	m.SetOverlay("https://r", overlayCfg("prod/web", "https://r/k8s/clusters/c-1", "new"))
	if _, ok := m.sessions["prod/web"]; ok {
		t.Fatal("session with the old token kept")
	}

	m.SetOverlay("https://r", nil)
	if m.raw.Contexts["prod/web"] != nil || m.raw.Contexts["prod/api"] == nil {
		t.Fatal("removing the overlay must only drop its contexts")
	}
}
