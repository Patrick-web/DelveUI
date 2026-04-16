package detect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jp/DelveUI/internal/config"
	"github.com/jp/DelveUI/internal/debugfiles"
)

// Service is exposed to the frontend via Wails bindings.
type Service struct {
	store *debugfiles.Store
}

func NewService(store *debugfiles.Store) *Service {
	return &Service{store: store}
}

// Scan discovers debug configs from known editor locations across the system.
func (s *Service) Scan() []DetectedSource {
	var roots []string
	// Also add paths from existing debug file entries
	for _, e := range s.store.List() {
		dir := filepath.Dir(e.Path)
		// Walk up to find project root (parent of .zed/.vscode)
		for _, sub := range []string{".zed", ".vscode"} {
			if filepath.Base(dir) == sub {
				dir = filepath.Dir(dir)
				break
			}
		}
		roots = append(roots, filepath.Dir(dir))
	}
	return Scan(roots)
}

// ScanDir scans a specific directory (user-chosen via folder picker).
func (s *Service) ScanDir(dir string) []DetectedSource {
	return Scan([]string{dir})
}

// Import imports a detected source's config file into the debug files store.
func (s *Service) Import(configPath string) error {
	_, err := s.store.Add(configPath)
	return err
}

// ImportConfigs creates a synthetic debug.json from detected configs and imports it.
// Used for GoLand configs which are spread across multiple XML files.
func (s *Service) ImportConfigs(projectPath string, editor string, configs []config.LaunchConfig) error {
	if len(configs) == 0 {
		return fmt.Errorf("no configs to import")
	}
	// Write a synthetic debug.json in the project's .delveui directory
	dir := filepath.Join(projectPath, ".delveui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	filename := fmt.Sprintf("imported-%s.json", editor)
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}

	_, err = s.store.Add(path)
	return err
}

// ImportAll imports all detected sources from a project.
func (s *Service) ImportAll(sources []DetectedSource) error {
	for _, src := range sources {
		if src.Editor == "GoLand" {
			// GoLand configs come from XML, need synthetic file
			if err := s.ImportConfigs(src.ProjectPath, "goland", src.Configs); err != nil {
				return err
			}
		} else {
			// Zed and VS Code have actual files
			if _, err := s.store.Add(src.ConfigPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// IsImported checks if a config path is already in the debug files store.
func (s *Service) IsImported(configPath string) bool {
	for _, e := range s.store.List() {
		if e.Path == configPath {
			return true
		}
	}
	return false
}

