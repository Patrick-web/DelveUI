package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/hujson"
)

type LaunchConfig struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	Adapter      string            `json:"adapter"`
	Request      string            `json:"request"`
	Mode         string            `json:"mode"`
	Program      string            `json:"program"`
	Cwd          string            `json:"cwd"`
	EnvFile      string            `json:"envFile"`
	Env          map[string]string `json:"env"`
	Args         []string          `json:"args"`
	BuildFlags   []string          `json:"buildFlags"`
	Disabled     bool              `json:"disabled,omitempty"`
	DisabledNote string            `json:"disabledNote,omitempty"`
	Language     string            `json:"language,omitempty"`
	// ProcessID is set when Request == "attach". Delve's DAP server expects
	// it as `processId` in the attach request body.
	ProcessID int `json:"processId,omitempty"`
	// EnvFiles records the dotenv files the discovery layer found via walk-up
	// (outermost → innermost). Their contents are already merged into Env at
	// launch time; this is preserved purely so the UI can show *which* files
	// contributed (the env inspector lists them as sources).
	EnvFiles []string `json:"envFiles,omitempty"`
}

// LoadFromWorkspace looks for debug configs in a directory, checking multiple
// editor locations: .zed/debug.json, .vscode/launch.json, .delveui/debug.json
func LoadFromWorkspace(dir string) (string, []LaunchConfig, error) {
	candidates := []string{
		filepath.Join(dir, ".zed", "debug.json"),
		filepath.Join(dir, ".vscode", "launch.json"),
		filepath.Join(dir, ".delveui", "debug.json"),
	}
	for _, path := range candidates {
		cfgs, err := LoadFile(path)
		if err == nil && len(cfgs) > 0 {
			return path, cfgs, nil
		}
	}
	return "", nil, fmt.Errorf("no debug config found in %s", dir)
}

// LoadFile loads debug configs from a JSON file. Supports both:
// - Zed format: bare JSON array of configs
// - VS Code format: { "version": "...", "configurations": [...] }
func LoadFile(path string) ([]LaunchConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	std, err := hujson.Standardize(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	// Try VS Code format first (object with configurations array)
	var vscodeFile struct {
		Configurations []LaunchConfig `json:"configurations"`
	}
	if json.Unmarshal(std, &vscodeFile) == nil && len(vscodeFile.Configurations) > 0 {
		cfgs := vscodeFile.Configurations
		for i := range cfgs {
			if cfgs[i].ID == "" {
				cfgs[i].ID = fmt.Sprintf("cfg-%d", i)
			}
		}
		expandTemplateVars(cfgs, projectRoot(path))
		return cfgs, nil
	}

	// Fall back to Zed format (bare array)
	var cfgs []LaunchConfig
	if err := json.Unmarshal(std, &cfgs); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	for i := range cfgs {
		if cfgs[i].ID == "" {
			cfgs[i].ID = fmt.Sprintf("cfg-%d", i)
		}
		// Mark non-Go configs as disabled
		adapter := cfgs[i].Adapter
		if adapter != "" && adapter != "Delve" && adapter != "delve" {
			cfgs[i].Disabled = true
			cfgs[i].DisabledNote = fmt.Sprintf("Only Go/Delve is supported (uses %q)", adapter)
			cfgs[i].Language = adapter
		}
		if cfgs[i].Language == "" {
			cfgs[i].Language = "go"
		}
	}

	// Expand editor template variables (Zed's $ZED_WORKTREE_ROOT, VS Code's
	// ${workspaceFolder}) so downstream code sees real filesystem paths.
	expandTemplateVars(cfgs, projectRoot(path))
	return cfgs, nil
}

// projectRoot returns the worktree/project root for a debug config file.
// If the config is in a dot-subdir (e.g. .zed/debug.json, .vscode/launch.json,
// .delveui/debug.json), the project root is the parent of that subdir.
// Otherwise it's the directory containing the file.
func projectRoot(configPath string) string {
	parent := filepath.Dir(configPath)
	base := filepath.Base(parent)
	if strings.HasPrefix(base, ".") {
		return filepath.Dir(parent)
	}
	return parent
}

// expandTemplateVars substitutes the editor template variables we understand
// into cfg string fields. Safe to run on any config — unknown files simply
// contain no matches.
//
// Variables handled:
//   - $ZED_WORKTREE_ROOT, ${ZED_WORKTREE_ROOT}  → workspace root (Zed)
//   - ${workspaceFolder}, ${workspaceRoot}       → workspace root (VS Code)
//   - ${userHome}                                → user's home dir (VS Code 1.69+)
//   - ${env:HOME}, $HOME                         → user's home dir (best-effort)
//   - leading ~                                   → user's home dir (shell)
//
// Unexpanded variables that reach `os.Stat` produce confusing errors like
// "open ${userHome}/foo: no such file or directory" — keep this list in sync
// with detect/parsers.go's resolveVSCodeVars.
func expandTemplateVars(cfgs []LaunchConfig, root string) {
	expand := func(s string) string { return ExpandPath(s, root) }
	for i := range cfgs {
		cfgs[i].Program = expand(cfgs[i].Program)
		cfgs[i].Cwd = expand(cfgs[i].Cwd)
		cfgs[i].EnvFile = expand(cfgs[i].EnvFile)
		for j := range cfgs[i].Args {
			cfgs[i].Args[j] = expand(cfgs[i].Args[j])
		}
		for j := range cfgs[i].BuildFlags {
			cfgs[i].BuildFlags[j] = expand(cfgs[i].BuildFlags[j])
		}
		for k, v := range cfgs[i].Env {
			cfgs[i].Env[k] = expand(v)
		}
	}
}

// ExpandPath performs editor template-variable + shell-style expansion on a
// single string. `root` is the workspace root used for ${workspaceFolder} /
// ${workspaceRoot} / $ZED_WORKTREE_ROOT — pass "" if not applicable. Use this
// at every system boundary that accepts a user-provided path: workspace open,
// debug-file open, file-tree navigation, etc.
func ExpandPath(s, root string) string {
	if s == "" {
		return s
	}
	home, _ := os.UserHomeDir()

	// ~ at the start expands to home (matches shell behaviour).
	if strings.HasPrefix(s, "~/") && home != "" {
		s = home + s[1:]
	} else if s == "~" && home != "" {
		s = home
	}

	pairs := []string{}
	if root != "" {
		pairs = append(pairs,
			"$ZED_WORKTREE_ROOT", root,
			"${ZED_WORKTREE_ROOT}", root,
			"${workspaceFolder}", root,
			"${workspaceRoot}", root,
		)
	}
	if home != "" {
		pairs = append(pairs,
			"${userHome}", home,
			"${env:HOME}", home,
			"$HOME", home,
		)
	}
	if len(pairs) == 0 {
		return s
	}
	return strings.NewReplacer(pairs...).Replace(s)
}
