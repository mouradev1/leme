package api

import (
	"context"
	"strings"
	"time"

	"leme/internal/platform/i18n"
	"leme/internal/platform/settings"
	"leme/internal/terminal"
	"leme/internal/update"
)

// AppAPI: app-level things (version, update notice, preferences).
type AppAPI struct{ d *Deps }

type Prefs struct {
	UpdateCheck       bool   `json:"updateCheck"`
	SkippedVersion    string `json:"skippedVersion"`
	NodeShellEnabled  bool   `json:"nodeShellEnabled"`
	NodeShellImage    string `json:"nodeShellImage"` // "" = default
	DefaultShellImage string `json:"defaultShellImage"`
}

func (a *AppAPI) Version() string { return a.d.Version }

func (a *AppAPI) Prefs() Prefs {
	s := settings.Load()
	return Prefs{
		UpdateCheck: !s.UpdateCheckDisabled, SkippedVersion: s.SkippedVersion,
		NodeShellEnabled: !s.NodeShellDisabled, NodeShellImage: s.NodeShellImage, DefaultShellImage: terminal.DefaultNodeShellImage,
	}
}

func (a *AppAPI) SetUpdateCheck(enabled bool) error {
	s := settings.Load()
	s.UpdateCheckDisabled = !enabled
	return settings.Save(s)
}

func (a *AppAPI) SkipVersion(version string) error {
	s := settings.Load()
	s.SkippedVersion = version
	return settings.Save(s)
}

// CheckUpdate asks GitHub for the latest release.
//
//   - auto=true (on startup): respects the user's choices. Returns
//     Available=false without any network request when the check is turned
//     off or this is a local development build, and hides a skipped version.
//   - auto=false ("Verificar agora"): always checks and reports.
func (a *AppAPI) CheckUpdate(auto bool) (update.Info, error) {
	s := settings.Load()
	info := update.Info{Current: a.d.Version}
	if auto && (s.UpdateCheckDisabled || update.IsDevBuild(a.d.Version)) {
		return info, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, err := update.New().Latest(ctx, a.d.Version)
	if err != nil {
		return info, err
	}
	if auto && info.Latest == s.SkippedVersion {
		info.Available = false
	}
	return info, nil
}

// SetNodeShell turns the node shell on/off and sets its image ("" = default).
func (a *AppAPI) SetNodeShell(enabled bool, image string) error {
	image = strings.TrimSpace(image)
	if image != "" {
		if err := terminal.ValidNodeShellImage(image); err != nil {
			return err
		}
	}
	s := settings.Load()
	s.NodeShellDisabled, s.NodeShellImage = !enabled, image
	return settings.Save(s)
}

// SetLanguage switches backend messages between Portuguese and English.
func (a *AppAPI) SetLanguage(lang string) { i18n.Set(lang) }
