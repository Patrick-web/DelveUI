package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tailscale/hujson"
)

type LaunchConfig struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Adapter    string            `json:"adapter"`
	Request    string            `json:"request"`
	Mode       string            `json:"mode"`
	Program    string            `json:"program"`
	Cwd        string            `json:"cwd"`
	EnvFile    string            `json:"envFile"`
	Env        map[string]string `json:"env"`
	Args       []string          `json:"args"`
	BuildFlags []string          `json:"buildFlags"`
}

// LoadFromWorkspace looks for <dir>/.zed/debug.json and returns parsed configs.
func LoadFromWorkspace(dir string) (string, []LaunchConfig, error) {
	path := filepath.Join(dir, ".zed", "debug.json")
	cfgs, err := LoadFile(path)
	return path, cfgs, err
}

func LoadFile(path string) ([]LaunchConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	std, err := hujson.Standardize(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var cfgs []LaunchConfig
	if err := json.Unmarshal(std, &cfgs); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	for i := range cfgs {
		if cfgs[i].ID == "" {
			cfgs[i].ID = fmt.Sprintf("cfg-%d", i)
		}
	}
	return cfgs, nil
}
