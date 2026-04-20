package updater

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	repo       = "Patrick-web/DelveUI"
	apiLatest  = "https://api.github.com/repos/" + repo + "/releases/latest"
	releaseURL = "https://github.com/" + repo + "/releases/latest"
)

type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseURL     string `json:"releaseUrl"`
	ReleaseNotes   string `json:"releaseNotes"`
}

type Service struct {
	version string
}

func NewService(version string) *Service {
	if version == "" {
		version = "0.0.0"
	}
	return &Service{version: version}
}

func (s *Service) CurrentVersion() string { return s.version }

// githubRelease matches the subset of GitHub's releases-API JSON we consume.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func fetchLatestRelease() (*githubRelease, error) {
	req, err := http.NewRequest("GET", apiLatest, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DelveUI-updater")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github: %s", resp.Status)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func (s *Service) CheckForUpdate() (UpdateInfo, error) {
	info := UpdateInfo{CurrentVersion: s.version}

	rel, err := fetchLatestRelease()
	if err != nil {
		return info, fmt.Errorf("update check failed: %w", err)
	}
	if rel.Draft || rel.Prerelease {
		return info, nil
	}

	latest, err := semver.Parse(trimV(rel.TagName))
	if err != nil {
		return info, fmt.Errorf("parse latest tag %q: %w", rel.TagName, err)
	}
	current, err := semver.Parse(trimV(s.version))
	if err != nil {
		// Dev builds have version="dev" or similar — treat any release as newer.
		info.Available = true
		info.LatestVersion = latest.String()
		info.ReleaseURL = rel.HTMLURL
		info.ReleaseNotes = rel.Body
		return info, nil
	}

	info.LatestVersion = latest.String()
	if latest.GT(current) {
		info.Available = true
		info.ReleaseURL = rel.HTMLURL
		info.ReleaseNotes = rel.Body
	}
	return info, nil
}

// ApplyUpdate opens the latest release page in the user's browser. In-place
// binary replacement is not attempted here: macOS .app bundles are signed +
// notarized and swapping the executable in-place breaks the bundle signature.
// Users should download the new installer.
func (s *Service) ApplyUpdate() (string, error) {
	return releaseURL, nil
}

func (s *Service) AppInfo() map[string]string {
	exe, _ := os.Executable()
	return map[string]string{
		"version":    s.version,
		"go":         runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"executable": exe,
	}
}

func trimV(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		return v[1:]
	}
	return v
}

// BackgroundCheck runs an update check after a delay. If a new version is found,
// emits an "update:available" Wails event so the frontend can show a toast.
func BackgroundCheck(app *application.App, version string, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		svc := NewService(version)
		info, err := svc.CheckForUpdate()
		if err != nil {
			log.Printf("update check: %v", err)
			return
		}
		if info.Available {
			log.Printf("update available: %s → %s (%s)", info.CurrentVersion, info.LatestVersion, info.ReleaseURL)
			if app != nil {
				app.Event.Emit("update:available", info)
			}
		}
	}()
}
