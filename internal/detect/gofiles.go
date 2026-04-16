package detect

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jp/DelveUI/internal/config"
)

type RunTarget struct {
	Label   string `json:"label"`
	Kind    string `json:"kind"` // "run" or "test"
	Package string `json:"package"`
	Dir     string `json:"dir"`
}

type FolderScanResult struct {
	EditorConfigs []DetectedSource `json:"editorConfigs"`
	RunTargets    []RunTarget      `json:"runTargets"`
	ProjectPath   string           `json:"projectPath"`
}

// FindRunTargets discovers Go main packages and test packages in a directory.
func FindRunTargets(dir string) []RunTarget {
	var targets []RunTarget
	targets = append(targets, findMainPackages(dir)...)
	targets = append(targets, findTestPackages(dir)...)
	return targets
}

func findMainPackages(root string) []RunTarget {
	// grep for "func main()" in .go files, excluding _test.go and vendor
	out, err := exec.Command("grep", "-rl", "--include=*.go", "--exclude=*_test.go",
		"--exclude-dir=vendor", "--exclude-dir=node_modules", "--exclude-dir=.git",
		"func main()", root).Output()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var targets []RunTarget
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dir := filepath.Dir(line)
		if seen[dir] {
			continue
		}
		seen[dir] = true

		rel, _ := filepath.Rel(root, dir)
		if rel == "" || rel == "." {
			rel = filepath.Base(root)
		}
		targets = append(targets, RunTarget{
			Label:   "go run ./" + rel,
			Kind:    "run",
			Package: "./" + rel,
			Dir:     dir,
		})
	}
	return targets
}

func findTestPackages(root string) []RunTarget {
	out, err := exec.Command("find", root, "-name", "*_test.go", "-type", "f",
		"-not", "-path", "*/vendor/*", "-not", "-path", "*/.git/*",
		"-not", "-path", "*/node_modules/*", "-maxdepth", "5").Output()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var targets []RunTarget
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dir := filepath.Dir(line)
		if seen[dir] {
			continue
		}
		seen[dir] = true

		rel, _ := filepath.Rel(root, dir)
		if rel == "" || rel == "." {
			rel = filepath.Base(root)
		}
		targets = append(targets, RunTarget{
			Label:   "go test ./" + rel,
			Kind:    "test",
			Package: "./" + rel,
			Dir:     dir,
		})
	}
	return targets
}

// RunTargetsToConfigs converts run targets into LaunchConfig entries.
func RunTargetsToConfigs(projectDir string, targets []RunTarget) []config.LaunchConfig {
	var cfgs []config.LaunchConfig
	for i, t := range targets {
		mode := "debug"
		if t.Kind == "test" {
			mode = "test"
		}
		cfgs = append(cfgs, config.LaunchConfig{
			ID:      fmt.Sprintf("auto-%d", i),
			Label:   t.Label,
			Adapter: "Delve",
			Request: "launch",
			Mode:    mode,
			Program: t.Dir,
			Cwd:     projectDir,
		})
	}
	return cfgs
}
