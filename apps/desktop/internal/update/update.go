// Package update tells the user when a newer Leme release exists.
//
// Privacy: it makes one anonymous GET to GitHub's public "latest release"
// endpoint (the same request a browser makes when opening the releases page).
// No usage data, cluster info or identifiers are sent. Users can turn it off.
// The request is made from Go, so the UI keeps its strict CSP (no network).
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// Repo is where releases are published.
const Repo = "mouradev1/leme"

// Info is what the UI shows in the "new version" dialog.
type Info struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`   // release page (opened in the browser)
	Notes     string `json:"notes"` // release notes (markdown, shown as plain text)
	Published string `json:"published"`
}

type Checker struct {
	BaseURL string // overridable in tests
	Client  *http.Client
}

func New() *Checker {
	return &Checker{BaseURL: "https://api.github.com", Client: &http.Client{Timeout: 8 * time.Second}}
}

// Latest fetches the latest published (non-draft, non-prerelease) release.
func (c *Checker) Latest(ctx context.Context, current string) (Info, error) {
	info := Info{Current: current}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/repos/"+Repo+"/releases/latest", nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Leme") // required by GitHub; no version/OS/user info
	resp, err := c.Client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return info, nil // no release published yet
	}
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("github: HTTP %d", resp.StatusCode)
	}
	var rel struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&rel); err != nil {
		return info, err
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	info.URL = rel.HTMLURL
	info.Notes = rel.Body
	info.Published = rel.PublishedAt
	info.Available = Newer(info.Latest, current)
	// Only ever send the user to this project's GitHub release pages.
	if !strings.HasPrefix(info.URL, "https://github.com/"+Repo+"/releases/") {
		info.URL = "https://github.com/" + Repo + "/releases/latest"
	}
	return info, nil
}

// Newer reports whether version a is newer than b ("1.2.3" form, optional
// leading v). Development builds ("dev", "x.y.z-dev") are never "newer" and
// are treated as older than any release.
func Newer(a, b string) bool {
	va, vb := "v"+strings.TrimPrefix(a, "v"), "v"+strings.TrimPrefix(b, "v")
	if !semver.IsValid(va) {
		return false
	}
	if !semver.IsValid(vb) {
		return true // e.g. "dev"
	}
	return semver.Compare(va, vb) > 0
}

// IsDevBuild: local builds don't nag on startup (manual check still works).
func IsDevBuild(v string) bool {
	return v == "" || v == "dev" || strings.HasSuffix(v, "-dev")
}
