package discovery

import (
	"os"
	"path/filepath"
	"strings"
)

// envFileNames lists the dotenv-style filenames we recognise during the
// walk-up search, in the order they should be picked up *within a single
// directory*. Later entries override earlier ones (matching dotenv-cli /
// Next.js conventions): a `.env.local` next to a `.env` wins.
var envFileNames = []string{
	".env",
	".env.local",
	".envrc", // direnv — we still parse it as plain dotenv (best effort)
}

// toolingDirNames are sibling subdirs we also peek into at each walk-up
// level. These cover common project-tooling conventions where the env file
// lives next to the package rather than on the walk-up path itself:
//
//   - .alis/         — Alis Build's per-project config dir
//   - .config/       — XDG-style per-project config
//   - config/        — generic
//   - .environment/  — some bespoke tooling
//   - env/           — some bespoke tooling
//
// We only look one level deep and only for the recognised env filenames;
// we don't recurse. New conventions can be added here without breaking
// existing layouts.
var toolingDirNames = []string{
	".alis",
	".config",
	"config",
	".environment",
	"env",
}

// FindEnvFiles walks from `dir` upward to (and including) `root`, collecting
// every recognised env file along the way. Output ordering: outermost first,
// innermost last — so the launcher applies them in order and the deepest
// override wins. If `dir` is not under `root`, only `dir` itself is searched.
//
// This is the "magic" the user asked for: zero-config, per-target env loading
// based on filesystem proximity. Files that don't exist are simply skipped;
// no error is returned for the common case of a project without env files.
func FindEnvFiles(dir, root string) []string {
	dir = filepath.Clean(dir)
	root = filepath.Clean(root)
	if dir == "" {
		return nil
	}

	// Collect dirs from the workspace root down to the target dir, so we
	// emit env files in outer→inner order (the launcher then merges them
	// in that order, letting the closest file win).
	var dirs []string
	cur := dir
	for {
		dirs = append([]string{cur}, dirs...)
		if cur == root || !strings.HasPrefix(cur, root) {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur { // hit filesystem root
			break
		}
		cur = parent
	}

	seen := make(map[string]struct{})
	var out []string
	add := func(p string) {
		if _, ok := seen[p]; ok {
			return
		}
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	// Per-level order:
	//  1. files directly in the dir (closest to the user's mental model)
	//  2. files inside well-known tooling sibling dirs (.alis/, .config/, …)
	//
	// Putting tooling dirs *after* in-dir files means a workspace-level
	// .alis/.env beats a workspace-level .env when both exist — which
	// matches Alis-style projects where .alis/.env is the canonical source
	// and a top-level .env, if present, is usually a fallback or example.
	for _, d := range dirs {
		for _, name := range envFileNames {
			add(filepath.Join(d, name))
		}
		for _, sub := range toolingDirNames {
			toolDir := filepath.Join(d, sub)
			if info, err := os.Stat(toolDir); err != nil || !info.IsDir() {
				continue
			}
			for _, name := range envFileNames {
				add(filepath.Join(toolDir, name))
			}
		}
	}
	return out
}
