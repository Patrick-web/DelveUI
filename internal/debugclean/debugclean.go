// Package debugclean removes Delve's auto-generated debug binaries
// (e.g. __debug_bin123456789) that accumulate in the project directory.
package debugclean

import (
	"os"
	"path/filepath"
	"strings"
)

// IsDebugBinary reports whether name looks like a Delve auto-generated
// debug binary (__debug_bin* with an optional .exe suffix on Windows).
func IsDebugBinary(name string) bool {
	return strings.HasPrefix(name, "__debug_bin")
}

// CleanDir removes __debug_bin* files directly inside dir (non-recursive).
// Returns absolute paths that were successfully removed.
func CleanDir(dir string) ([]string, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, e := range entries {
		if e.IsDir() || !IsDebugBinary(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if err := os.Remove(p); err == nil {
			removed = append(removed, p)
		}
	}
	return removed, nil
}

var skipDirs = map[string]bool{
	".git":        true,
	"node_modules": true,
	"vendor":      true,
	".idea":       true,
	".vscode":     true,
	".zed":        true,
	".delveui":    true,
	"dist":        true,
	"build":       true,
	"__pycache__": true,
}

// CleanRecursive walks root and removes every __debug_bin* file,
// skipping common build/vendor/hidden directories.
func CleanRecursive(root string) ([]string, error) {
	if root == "" {
		return nil, nil
	}
	var removed []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (skipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if IsDebugBinary(d.Name()) {
			if os.Remove(path) == nil {
				removed = append(removed, path)
			}
		}
		return nil
	})
	return removed, err
}
