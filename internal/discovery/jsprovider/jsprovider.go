// Package jsprovider implements discovery.Provider for JavaScript/TypeScript
// projects. It finds entry points (main.js, index.js, package.json main field)
// and running Node.js processes for attach.
package jsprovider

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/jp/DelveUI/internal/config"
	"github.com/jp/DelveUI/internal/discovery"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string { return "node" }

func (p *Provider) Discover(ctx context.Context, root string) ([]discovery.Target, error) {
	return p.findEntryPoints(ctx, root), nil
}

func (p *Provider) ToLaunchConfig(t discovery.Target) config.LaunchConfig {
	mode := "debug"
	if t.Kind == discovery.KindAttach {
		mode = "local"
	}
	cfg := config.LaunchConfig{
		ID:       t.ID,
		Label:    t.Label,
		Adapter:  "JavaScript",
		Request:  "launch",
		Mode:     mode,
		Program:  t.Program,
		Cwd:      t.Dir,
		Args:     append([]string(nil), t.Args...),
		Language: "node",
	}
	if t.Kind == discovery.KindAttach {
		cfg.Request = "attach"
	}
	return cfg
}

func stableID(parts ...string) string {
	h := sha1.New()
	for _, s := range parts {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return "js-" + hex.EncodeToString(h.Sum(nil))[:12]
}

func (p *Provider) findEntryPoints(ctx context.Context, root string) []discovery.Target {
	// Find .js, .mjs, .cjs files near the root.
	cmd := exec.CommandContext(ctx, "find", root,
		"-maxdepth", "3",
		"(", "-name", "*.js", "-o", "-name", "*.mjs", "-o", "-name", "*.cjs", ")",
		"-type", "f",
		"-not", "-path", "*/node_modules/*",
		"-not", "-path", "*/.git/*",
		"-not", "-path", "*/.delveui/*",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var targets []discovery.Target
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dir := filepath.Dir(line)
		fileOnly := filepath.Base(line)

		if seen[fileOnly] {
			continue
		}
		seen[fileOnly] = true

		rel, _ := filepath.Rel(root, fileOnly)
		if rel == "" || rel == "." {
			rel = fileOnly
		}
		targets = append(targets, discovery.Target{
			ID:          stableID("run", fileOnly),
			Provider:    "node",
			Kind:        discovery.KindRun,
			Label:       rel,
			Description: fileOnly,
			Dir:         dir,
			Program:     line,
			SourceFile:  line,
		})
	}
	return targets
}

func (p *Provider) DiscoverProcesses(ctx context.Context, root string) ([]discovery.Target, error) {
	if runtime.GOOS == "windows" {
		return nil, nil
	}
	cmd := exec.CommandContext(ctx, "ps", "-axo", "pid=,comm=,args=")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	root = filepath.Clean(root)
	var targets []discovery.Target
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		comm := fields[1]
		base := filepath.Base(comm)
		if base != "node" {
			continue
		}
		// Skip tools and debug servers
		if strings.Contains(comm, "debug") || strings.Contains(comm, "eslint") {
			continue
		}
		targets = append(targets, discovery.Target{
			ID:          stableID("attach", strconv.Itoa(pid), comm),
			Provider:    "node",
			Kind:        discovery.KindAttach,
			Label:       fmt.Sprintf("attach: %s", base),
			Description: fmt.Sprintf("PID %d", pid),
			Dir:         root,
			PID:         pid,
		})
	}
	return targets, nil
}
