package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestFindEnvFiles_WalkUp verifies that an env file at every ancestor on the
// walk-up path is collected, outermost first. This is the typical layout —
// project-root .env plus optional package-level overrides.
func TestFindEnvFiles_WalkUp(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), "TOP=1")

	// .env directly on the walk-up path.
	mid := filepath.Join(root, "services")
	if err := os.MkdirAll(mid, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(mid, ".env"), "MID=1")

	targetDir := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := FindEnvFiles(targetDir, root)
	want := []string{
		filepath.Join(root, ".env"),
		filepath.Join(mid, ".env"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected env files\n got: %v\nwant: %v", got, want)
	}
}

// TestFindEnvFiles_ToolingDir verifies the voyage-style layout: an env file
// inside a well-known tooling subdir (.alis/) at the workspace root is
// picked up even though it's a sibling of the target's walk-up path.
func TestFindEnvFiles_ToolingDir(t *testing.T) {
	root := t.TempDir()
	alis := filepath.Join(root, ".alis")
	if err := os.MkdirAll(alis, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(alis, ".env"), "ALIS=1")

	targetDir := filepath.Join(root, "hubspot", "v1", "cmd", "foo")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := FindEnvFiles(targetDir, root)
	want := []string{filepath.Join(alis, ".env")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected tooling-dir env file\n got: %v\nwant: %v", got, want)
	}
}

// TestFindEnvFiles_ToolingDirOrdering: tooling-dir env at workspace root
// comes after the workspace .env (so .alis/.env wins over a fallback .env).
// A package-level .env on the walk-up path comes last (closest wins overall).
func TestFindEnvFiles_ToolingDirOrdering(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), "ROOT=1")

	alis := filepath.Join(root, ".alis")
	if err := os.MkdirAll(alis, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(alis, ".env"), "ALIS=1")

	pkg := filepath.Join(root, "service")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(pkg, ".env"), "PKG=1")

	got := FindEnvFiles(pkg, root)
	want := []string{
		filepath.Join(root, ".env"),
		filepath.Join(alis, ".env"),
		filepath.Join(pkg, ".env"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected order\n got: %v\nwant: %v", got, want)
	}
}

// TestFindEnvFiles_NoFiles returns an empty list when no env files exist —
// the common case for fresh projects.
func TestFindEnvFiles_NoFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cmd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindEnvFiles(dir, root); len(got) != 0 {
		t.Errorf("expected no env files, got %v", got)
	}
}

// TestFindEnvFiles_PrecedenceWithinDir confirms that within a single dir,
// .env.local is emitted after .env (so the local file wins).
func TestFindEnvFiles_PrecedenceWithinDir(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".env"), "X=base")
	mustWrite(t, filepath.Join(root, ".env.local"), "X=local")

	got := FindEnvFiles(root, root)
	want := []string{
		filepath.Join(root, ".env"),
		filepath.Join(root, ".env.local"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected order\n got: %v\nwant: %v", got, want)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
