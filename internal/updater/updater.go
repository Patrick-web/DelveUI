package updater

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const repo = "Patrick-web/DelveUI"

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

func (s *Service) CurrentVersion() string {
	return s.version
}

func (s *Service) CheckForUpdate() (UpdateInfo, error) {
	info := UpdateInfo{CurrentVersion: s.version}

	latest, found, err := selfupdate.DetectLatest(repo)
	if err != nil {
		return info, fmt.Errorf("update check failed: %w", err)
	}
	if !found {
		return info, nil
	}

	v := semver.MustParse(trimV(s.version))
	if latest.Version.LTE(v) {
		info.LatestVersion = latest.Version.String()
		return info, nil
	}

	info.Available = true
	info.LatestVersion = latest.Version.String()
	info.ReleaseURL = latest.URL
	info.ReleaseNotes = latest.ReleaseNotes
	return info, nil
}

func (s *Service) ApplyUpdate() (string, error) {
	v := semver.MustParse(trimV(s.version))
	latest, err := selfupdate.UpdateSelf(v, repo)
	if err != nil {
		return "", fmt.Errorf("update failed: %w", err)
	}
	return latest.Version.String(), nil
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
	if len(v) > 0 && v[0] == 'v' {
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
