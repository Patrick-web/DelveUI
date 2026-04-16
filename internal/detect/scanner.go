package detect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jp/DelveUI/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type DetectedSource struct {
	Editor      string                `json:"editor"`
	ProjectPath string                `json:"projectPath"`
	ConfigPath  string                `json:"configPath"`
	ConfigCount int                   `json:"configCount"`
	Configs     []config.LaunchConfig `json:"configs"`
}

// pruneNames returns exact directory names to skip via find -prune.
// Using -name (not -path) avoids substring matches like alis.build matching "build".
func pruneNames() []string {
	return []string{
		// OS / system
		"Library", ".Trash", "Applications", "Desktop",
		"Documents", "Downloads", "Pictures", "Music", "Movies",
		"Public", "Dropbox", "OneDrive",
		// Caches
		".cache", ".Spotlight-V100", ".fseventsd", ".TemporaryItems",
		// Package managers
		"node_modules", ".npm", ".yarn", ".pnpm-store",
		".cargo", ".rustup", ".rbenv", ".pyenv", ".nvm", ".bun", ".deno",
		// Go
		".go",
		// VCS
		".git", "vendor", "__pycache__",
		// Containers
		".docker", ".colima", ".lima",
		// IDE remote
		".vscode-server", ".cursor-server",
		// Other
		"Pods", ".gradle", ".m2",
	}
}

// buildFindArgs constructs a find command using -prune for exact dir name exclusion.
// pattern is the search target, e.g. "-name go.mod" or "-path */.zed/debug.json".
func buildFindArgs(root string, patternFlag string, patternVal string, maxDepth int) []string {
	prunes := pruneNames()
	args := []string{root, "-maxdepth", fmt.Sprintf("%d", maxDepth)}

	// Build prune expression: \( -name X -o -name Y ... \) -prune
	if len(prunes) > 0 {
		args = append(args, "(")
		for i, name := range prunes {
			if i > 0 {
				args = append(args, "-o")
			}
			args = append(args, "-name", name)
		}
		args = append(args, ")", "-prune", "-o")
	}

	args = append(args, patternFlag, patternVal, "-type", "f", "-print")
	return args
}

// Scan uses two strategies and emits progress events via the Wails app.
func Scan(app *application.App, extraRoots []string) []DetectedSource {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}

	roots := []string{home}
	for _, r := range extraRoots {
		if r != "" && r != home {
			roots = append(roots, r)
		}
	}

	seen := make(map[string]bool)
	var results []DetectedSource
	var mu sync.Mutex
	found := 0

	emit := func(phase, dir string) {
		if app != nil {
			app.Event.Emit("scan:progress", map[string]any{"phase": phase, "dir": dir, "found": found})
		}
	}
	emitResult := func(s DetectedSource) {
		if app != nil {
			app.Event.Emit("scan:result", s)
		}
	}

	addResult := func(s DetectedSource) {
		mu.Lock()
		defer mu.Unlock()
		if seen[s.ConfigPath] {
			return
		}
		seen[s.ConfigPath] = true
		results = append(results, s)
		found++
		emitResult(s)
	}

	// Strategy 1: find go.mod projects → check for editor configs
	for _, root := range roots {
		emit("searching", shortDir(root, home))
		projectDirs := findGoProjects(root)
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		for _, dir := range projectDirs {
			mu.Lock()
			skip := seen[dir]
			if !skip {
				seen[dir+"__project"] = true
			}
			mu.Unlock()
			if skip {
				continue
			}
			wg.Add(1)
			go func(dir string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				emit("checking", shortDir(dir, home))
				for _, s := range scanProject(dir) {
					addResult(s)
				}
			}(dir)
		}
		wg.Wait()
	}

	// Strategy 2: find debug config files directly (monorepos)
	for _, root := range roots {
		emit("searching", shortDir(root, home)+" (direct)")
		for _, s := range findConfigsDirect(root) {
			addResult(s)
		}
	}

	if app != nil {
		app.Event.Emit("scan:done", map[string]any{"total": found})
	}
	return results
}

// ScanSync is a non-streaming version for simple use.
func ScanSync(extraRoots []string) []DetectedSource {
	return Scan(nil, extraRoots)
}

func shortDir(dir, home string) string {
	if home != "" && strings.HasPrefix(dir, home) {
		return "~" + dir[len(home):]
	}
	return dir
}

func findGoProjects(root string) []string {
	args := buildFindArgs(root, "-name", "go.mod", 6)
	out, err := exec.Command("find", args...).Output()
	if err != nil {
		return nil
	}
	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			dirs = append(dirs, filepath.Dir(line))
		}
	}
	return dirs
}

func findConfigsDirect(root string) []DetectedSource {
	var results []DetectedSource

	// .zed/debug.json
	args := buildFindArgs(root, "-path", "*/.zed/debug.json", 6)
	if out, err := exec.Command("find", args...).Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			projectDir := filepath.Dir(filepath.Dir(line))
			if cfgs, err := parseZed(line); err == nil && len(cfgs) > 0 {
				results = append(results, DetectedSource{
					Editor: "Zed", ProjectPath: projectDir, ConfigPath: line,
					ConfigCount: len(cfgs), Configs: cfgs,
				})
			}
		}
	}

	// .vscode/launch.json
	args = buildFindArgs(root, "-path", "*/.vscode/launch.json", 6)
	if out, err := exec.Command("find", args...).Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			projectDir := filepath.Dir(filepath.Dir(line))
			if cfgs, err := parseVSCode(line, projectDir); err == nil && len(cfgs) > 0 {
				results = append(results, DetectedSource{
					Editor: "VS Code", ProjectPath: projectDir, ConfigPath: line,
					ConfigCount: len(cfgs), Configs: cfgs,
				})
			}
		}
	}

	return results
}

func scanProject(dir string) []DetectedSource {
	var sources []DetectedSource

	zedPath := filepath.Join(dir, ".zed", "debug.json")
	if cfgs, err := parseZed(zedPath); err == nil && len(cfgs) > 0 {
		sources = append(sources, DetectedSource{
			Editor: "Zed", ProjectPath: dir, ConfigPath: zedPath,
			ConfigCount: len(cfgs), Configs: cfgs,
		})
	}

	vscodePath := filepath.Join(dir, ".vscode", "launch.json")
	if cfgs, err := parseVSCode(vscodePath, dir); err == nil && len(cfgs) > 0 {
		sources = append(sources, DetectedSource{
			Editor: "VS Code", ProjectPath: dir, ConfigPath: vscodePath,
			ConfigCount: len(cfgs), Configs: cfgs,
		})
	}

	ideaDir := filepath.Join(dir, ".idea", "runConfigurations")
	if cfgs, err := parseGoLand(ideaDir, dir); err == nil && len(cfgs) > 0 {
		sources = append(sources, DetectedSource{
			Editor: "GoLand", ProjectPath: dir, ConfigPath: ideaDir,
			ConfigCount: len(cfgs), Configs: cfgs,
		})
	}

	return sources
}
