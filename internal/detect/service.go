package detect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/jp/DelveUI/internal/config"
	"github.com/jp/DelveUI/internal/debugfiles"
)

// Service is exposed to the frontend via Wails bindings.
type Service struct {
	store *debugfiles.Store
	app   *application.App
}

func NewService(store *debugfiles.Store) *Service {
	return &Service{store: store}
}

func (s *Service) SetApp(app *application.App) { s.app = app }

// Scan discovers debug configs from known editor locations across the system.
func (s *Service) Scan() []DetectedSource {
	var roots []string
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
	return Scan(s.app, roots)
}

// ScanDir scans a specific directory (user-chosen via folder picker).
func (s *Service) ScanDir(dir string) []DetectedSource {
	return Scan(s.app, []string{dir})
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

// ScanFolder scans a specific folder for editor configs and Go run/test targets.
func (s *Service) ScanFolder(dir string) FolderScanResult {
	result := FolderScanResult{ProjectPath: dir}
	result.EditorConfigs = scanProject(dir)
	result.RunTargets = FindRunTargets(dir)
	return result
}

// PickAndScanFolder opens a native folder picker and scans the chosen
// directory for editor configs (Zed/VSCode/GoLand). Used by the import UI
// to let users add folders by hand alongside the system-wide scan.
//
// Returns ProjectPath="" with no editor configs if the user cancelled. A
// folder with zero configs returns ProjectPath set but EditorConfigs nil —
// the UI surfaces this as "no configs found, open as workspace anyway?".
func (s *Service) PickAndScanFolder() (FolderScanResult, error) {
	if s.app == nil {
		return FolderScanResult{}, fmt.Errorf("app not initialized")
	}
	dialog := s.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                "Choose folder to scan for debug configs",
		CanChooseFiles:       false,
		CanChooseDirectories: true,
	})
	path, err := dialog.PromptForSingleSelection()
	if err != nil || path == "" {
		return FolderScanResult{}, err
	}
	return s.ScanFolder(path), nil
}

// CreateConfigFromTargets writes a synthetic debug.json from run targets and imports it.
func (s *Service) CreateConfigFromTargets(projectDir string, targets []RunTarget) error {
	if len(targets) == 0 {
		return fmt.Errorf("no targets")
	}
	cfgs := RunTargetsToConfigs(projectDir, targets)
	dir := filepath.Join(projectDir, ".delveui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "debug.json")
	data, _ := json.MarshalIndent(cfgs, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	_, err := s.store.Add(path)
	return err
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

