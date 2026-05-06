// Package goprovider implements discovery.Provider for Go projects.
// It finds main packages (run targets), test packages (test targets), and
// running Go processes (attach targets). Detection is grep/ps-based to keep
// startup latency low; gopls integration could replace it later for accuracy.
package goprovider

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"maps"
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

func (p *Provider) Name() string { return "go" }

func (p *Provider) Discover(ctx context.Context, root string) ([]discovery.Target, error) {
	var out []discovery.Target
	out = append(out, p.findMainPackages(ctx, root)...)
	out = append(out, p.findTestPackages(ctx, root)...)
	return out, nil
}

func (p *Provider) ToLaunchConfig(t discovery.Target) config.LaunchConfig {
	mode := "debug"
	switch t.Kind {
	case discovery.KindTest, discovery.KindBenchmark:
		mode = "test"
	case discovery.KindAttach:
		mode = "local"
	}
	cfg := config.LaunchConfig{
		ID:       t.ID,
		Label:    t.Label,
		Adapter:  "Delve",
		Request:  "launch",
		Mode:     mode,
		Program:  t.Program,
		Cwd:      t.Dir,
		Args:     append([]string(nil), t.Args...),
		Language: "go",
	}
	if t.Kind == discovery.KindAttach {
		cfg.Request = "attach"
	}
	if len(t.Env) > 0 {
		cfg.Env = make(map[string]string, len(t.Env))
		maps.Copy(cfg.Env, t.Env)
	}
	return cfg
}

// stableID hashes the parts that identify a logical target so the ID survives
// rescans (the UI uses it to track running state across refreshes).
func stableID(parts ...string) string {
	h := sha1.New()
	for _, s := range parts {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return "go-" + hex.EncodeToString(h.Sum(nil))[:12]
}

// findMainPackages locates every directory containing a `func main()` in a
// non-test .go file. We grep rather than parse Go syntax because the result
// is identical for the 99% case and ~50× faster on big monorepos.
func (p *Provider) findMainPackages(ctx context.Context, root string) []discovery.Target {
	cmd := exec.CommandContext(ctx, "grep", "-rl",
		"--include=*.go", "--exclude=*_test.go",
		"--exclude-dir=vendor", "--exclude-dir=node_modules",
		"--exclude-dir=.git", "--exclude-dir=.delveui",
		"^func main()", root)
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
		if seen[dir] {
			continue
		}
		seen[dir] = true

		rel, _ := filepath.Rel(root, dir)
		if rel == "" || rel == "." {
			rel = filepath.Base(root)
		}
		label := rel
		// Pick a more meaningful label for the common cmd/<name>/main.go layout.
		if strings.HasPrefix(rel, "cmd/") {
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) >= 2 {
				label = parts[1]
			}
		}
		targets = append(targets, discovery.Target{
			ID:          stableID("run", dir),
			Provider:    "go",
			Kind:        discovery.KindRun,
			Label:       label,
			Description: "./" + rel,
			Dir:         dir,
			Program:     dir,
			SourceFile:  line,
		})
	}
	return targets
}

// findTestPackages locates every directory containing a *_test.go file and
// emits one test target per package. We deliberately don't drill down to
// individual `func TestXxx` to keep the list scannable; running a single
// test will be a future enhancement (the UI is structurally ready for it).
func (p *Provider) findTestPackages(ctx context.Context, root string) []discovery.Target {
	cmd := exec.CommandContext(ctx, "find", root,
		"-maxdepth", "8",
		"-name", "*_test.go", "-type", "f",
		"-not", "-path", "*/vendor/*",
		"-not", "-path", "*/.git/*",
		"-not", "-path", "*/node_modules/*",
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
		if seen[dir] {
			continue
		}
		seen[dir] = true

		rel, _ := filepath.Rel(root, dir)
		if rel == "" || rel == "." {
			rel = filepath.Base(root)
		}
		targets = append(targets, discovery.Target{
			ID:          stableID("test", dir),
			Provider:    "go",
			Kind:        discovery.KindTest,
			Label:       "test: " + rel,
			Description: "./" + rel,
			Dir:         dir,
			Program:     dir,
			SourceFile:  line,
		})
	}
	return targets
}

// DiscoverProcesses lists running processes that look like Go binaries built
// from this workspace. Heuristic: process command path resides under root, OR
// the binary contains common Go runtime markers in its argv. Surfaced in the
// UI's attach group; the manual "Attach to PID…" picker is the fallback.
func (p *Provider) DiscoverProcesses(ctx context.Context, root string) ([]discovery.Target, error) {
	if runtime.GOOS == "windows" {
		// ps-based path is Unix-only. Windows attach falls back to the
		// manual picker until we wire tasklist.exe.
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
		// Manual split: pid <whitespace> comm <whitespace> args(rest)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		comm := fields[1]
		// Skip common noise: kernel, helpers, our own dlv, our own DelveUI.
		base := filepath.Base(comm)
		if base == "dlv" || base == "DelveUI" || base == "delveui" {
			continue
		}
		// Only surface processes whose binary appears to live under the
		// workspace root (or whose argv starts with a path under it).
		isWorkspaceProc := strings.HasPrefix(comm, root+string(filepath.Separator)) || strings.HasPrefix(comm, root)
		if !isWorkspaceProc {
			// Also check first arg in case `comm` is a short name.
			if len(fields) >= 3 {
				if !strings.HasPrefix(fields[2], root) {
					continue
				}
			} else {
				continue
			}
		}
		args := ""
		if _, after, ok := strings.Cut(line, comm); ok {
			args = strings.TrimSpace(after)
		}
		targets = append(targets, discovery.Target{
			ID:          stableID("attach", strconv.Itoa(pid), comm),
			Provider:    "go",
			Kind:        discovery.KindAttach,
			Label:       fmt.Sprintf("attach: %s", base),
			Description: fmt.Sprintf("PID %d  %s", pid, args),
			Dir:         root,
			PID:         pid,
		})
	}
	return targets, nil
}
