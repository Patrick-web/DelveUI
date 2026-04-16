package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jp/DelveUI/internal/config"
)

type DetectedSource struct {
	Editor      string                `json:"editor"`
	ProjectPath string                `json:"projectPath"`
	ConfigPath  string                `json:"configPath"`
	ConfigCount int                   `json:"configCount"`
	Configs     []config.LaunchConfig `json:"configs"`
}

// Scan uses system-level search to find all Go projects with editor debug configs
// across the entire home directory. Uses `find` for speed and filters efficiently.
func Scan(extraRoots []string) []DetectedSource {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}

	// Use mdfind on macOS for fast indexed search, fall back to find
	var projectDirs []string
	projectDirs = append(projectDirs, findGoProjectsFast(home)...)
	for _, r := range extraRoots {
		if r != "" && r != home {
			projectDirs = append(projectDirs, findGoProjectsFast(r)...)
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, d := range projectDirs {
		if !seen[d] {
			seen[d] = true
			unique = append(unique, d)
		}
	}

	// Scan each project in parallel
	var mu sync.Mutex
	var results []DetectedSource
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // limit concurrency

	for _, dir := range unique {
		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sources := scanProject(dir)
			if len(sources) > 0 {
				mu.Lock()
				results = append(results, sources...)
				mu.Unlock()
			}
		}(dir)
	}
	wg.Wait()
	return results
}

// findGoProjectsFast uses system commands to locate go.mod files quickly.
func findGoProjectsFast(root string) []string {
	// Directories to skip
	excludes := []string{
		"node_modules", ".git", "vendor", ".cache", "Library",
		".Trash", "Applications", ".npm", ".cargo", ".rustup",
		"go/pkg", ".local/share", "Pictures", "Music", "Movies",
	}

	args := []string{root, "-name", "go.mod", "-type", "f", "-maxdepth", "6"}
	for _, ex := range excludes {
		args = append(args, "-not", "-path", "*/"+ex+"/*")
	}

	out, err := exec.Command("find", args...).Output()
	if err != nil {
		return nil
	}

	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dirs = append(dirs, filepath.Dir(line))
	}
	return dirs
}

func scanProject(dir string) []DetectedSource {
	var sources []DetectedSource
	seen := make(map[string]bool)

	// Zed
	zedPath := filepath.Join(dir, ".zed", "debug.json")
	if cfgs, err := parseZed(zedPath); err == nil && len(cfgs) > 0 {
		if !seen[zedPath] {
			seen[zedPath] = true
			sources = append(sources, DetectedSource{
				Editor: "Zed", ProjectPath: dir, ConfigPath: zedPath,
				ConfigCount: len(cfgs), Configs: cfgs,
			})
		}
	}

	// VS Code / Cursor
	vscodePath := filepath.Join(dir, ".vscode", "launch.json")
	if cfgs, err := parseVSCode(vscodePath, dir); err == nil && len(cfgs) > 0 {
		if !seen[vscodePath] {
			seen[vscodePath] = true
			sources = append(sources, DetectedSource{
				Editor: "VS Code", ProjectPath: dir, ConfigPath: vscodePath,
				ConfigCount: len(cfgs), Configs: cfgs,
			})
		}
	}

	// GoLand / JetBrains
	ideaDir := filepath.Join(dir, ".idea", "runConfigurations")
	if cfgs, err := parseGoLand(ideaDir, dir); err == nil && len(cfgs) > 0 {
		if !seen[ideaDir] {
			seen[ideaDir] = true
			sources = append(sources, DetectedSource{
				Editor: "GoLand", ProjectPath: dir, ConfigPath: ideaDir,
				ConfigCount: len(cfgs), Configs: cfgs,
			})
		}
	}

	return sources
}
