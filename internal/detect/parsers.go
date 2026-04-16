package detect

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jp/DelveUI/internal/config"
	"github.com/tailscale/hujson"
)

// --- Zed ---

func parseZed(path string) ([]config.LaunchConfig, error) {
	return config.LoadFile(path)
}

// --- VS Code / Cursor ---

type vscodeLaunchFile struct {
	Version        string            `json:"version"`
	Configurations []vscodeConfig    `json:"configurations"`
}

type vscodeConfig struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Request    string            `json:"request"`
	Mode       string            `json:"mode"`
	Program    string            `json:"program"`
	Cwd        string            `json:"cwd"`
	Env        map[string]string `json:"env"`
	EnvFile    string            `json:"envFile"`
	Args       []string          `json:"args"`
	BuildFlags string            `json:"buildFlags"`
}

func parseVSCode(path string, projectDir string) ([]config.LaunchConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// VS Code launch.json may have comments
	std, err := hujson.Standardize(raw)
	if err != nil {
		return nil, err
	}
	var file vscodeLaunchFile
	if err := json.Unmarshal(std, &file); err != nil {
		return nil, err
	}

	var cfgs []config.LaunchConfig
	for i, vc := range file.Configurations {
		// Only import Go/Delve configs
		if vc.Type != "go" && vc.Type != "dlv" && vc.Type != "" {
			continue
		}
		// Resolve ${workspaceFolder}
		program := resolveVSCodeVars(vc.Program, projectDir)
		cwd := resolveVSCodeVars(vc.Cwd, projectDir)
		envFile := resolveVSCodeVars(vc.EnvFile, projectDir)

		label := vc.Name
		if label == "" {
			label = fmt.Sprintf("Config %d", i+1)
		}

		var buildFlags []string
		if vc.BuildFlags != "" {
			buildFlags = strings.Fields(vc.BuildFlags)
		}

		cfgs = append(cfgs, config.LaunchConfig{
			ID:         fmt.Sprintf("vscode-%d", i),
			Label:      label,
			Adapter:    "Delve",
			Request:    or(vc.Request, "launch"),
			Mode:       or(vc.Mode, "debug"),
			Program:    program,
			Cwd:        cwd,
			EnvFile:    envFile,
			Env:        vc.Env,
			Args:       vc.Args,
			BuildFlags: buildFlags,
		})
	}
	return cfgs, nil
}

func resolveVSCodeVars(s string, projectDir string) string {
	s = strings.ReplaceAll(s, "${workspaceFolder}", projectDir)
	s = strings.ReplaceAll(s, "${workspaceRoot}", projectDir)
	home, _ := os.UserHomeDir()
	if home != "" {
		s = strings.ReplaceAll(s, "${userHome}", home)
	}
	return s
}

// --- GoLand / JetBrains ---

type jetbrainsComponent struct {
	XMLName       xml.Name                  `xml:"component"`
	Configuration jetbrainsRunConfiguration `xml:"configuration"`
}

type jetbrainsRunConfiguration struct {
	Name       string                `xml:"name,attr"`
	Type       string                `xml:"type,attr"`
	Module     jetbrainsOption       `xml:"module"`
	WorkingDir jetbrainsOption       `xml:"working_directory"`
	Kind       jetbrainsOption       `xml:"kind"`
	Package    jetbrainsOption       `xml:"package"`
	Directory  jetbrainsOption       `xml:"directory"`
	FilePath   jetbrainsOption       `xml:"filePath"`
	Envs       jetbrainsEnvs         `xml:"envs"`
	Parameters jetbrainsOption       `xml:"parameters"`
}

type jetbrainsOption struct {
	Value string `xml:"value,attr"`
}

type jetbrainsEnvs struct {
	Envs []jetbrainsEnv `xml:"env"`
}

type jetbrainsEnv struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func parseGoLand(dir string, projectDir string) ([]config.LaunchConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var cfgs []config.LaunchConfig
	for i, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".xml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var comp jetbrainsComponent
		if xml.Unmarshal(data, &comp) != nil {
			continue
		}
		rc := comp.Configuration
		// Filter for Go configs
		if !strings.Contains(strings.ToLower(rc.Type), "go") {
			continue
		}

		program := rc.Package.Value
		if program == "" {
			program = rc.Directory.Value
		}
		if program == "" {
			program = rc.FilePath.Value
		}
		if program == "" {
			program = projectDir
		}

		env := make(map[string]string)
		for _, e := range rc.Envs.Envs {
			env[e.Name] = e.Value
		}

		label := rc.Name
		if label == "" {
			label = fmt.Sprintf("GoLand %d", i+1)
		}

		var args []string
		if rc.Parameters.Value != "" {
			args = strings.Fields(rc.Parameters.Value)
		}

		cfgs = append(cfgs, config.LaunchConfig{
			ID:      fmt.Sprintf("goland-%d", i),
			Label:   label,
			Adapter: "Delve",
			Request: "launch",
			Mode:    "debug",
			Program: program,
			Cwd:     or(rc.WorkingDir.Value, projectDir),
			Env:     env,
			Args:    args,
		})
	}
	return cfgs, nil
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
