package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsGitWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()

	// main repo: .git is a directory → not a worktree
	mainDir := filepath.Join(tmp, "main")
	must(t, os.MkdirAll(mainDir, 0o755))
	runGit(t, mainDir, "init")
	if isGitWorktree(mainDir) {
		t.Error("main repo with .git directory should not be a worktree")
	}

	// create an initial commit so worktree add works
	runGit(t, mainDir, "config", "user.email", "test@test.com")
	runGit(t, mainDir, "config", "user.name", "test")
	must(t, os.WriteFile(filepath.Join(mainDir, "README.md"), []byte("test"), 0o644))
	runGit(t, mainDir, "add", "README.md")
	runGit(t, mainDir, "commit", "-m", "init")

	// worktree: .git is a file containing "gitdir:" → is a worktree
	wtDir := filepath.Join(tmp, "worktree")
	runGit(t, mainDir, "worktree", "add", wtDir)
	if !isGitWorktree(wtDir) {
		t.Error("worktree with .git file should be detected")
	}

	// plain directory → not a worktree
	plainDir := filepath.Join(tmp, "plain")
	must(t, os.MkdirAll(plainDir, 0o755))
	if isGitWorktree(plainDir) {
		t.Error("plain directory should not be a worktree")
	}
}

func TestScanProjectSkipsWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()

	mainDir := filepath.Join(tmp, "main")
	must(t, os.MkdirAll(mainDir, 0o755))
	runGit(t, mainDir, "init")
	runGit(t, mainDir, "config", "user.email", "test@test.com")
	runGit(t, mainDir, "config", "user.name", "test")

	// go.mod (needed because scanProject is only called for Go projects in
	// strategy 1, but we test it standalone)
	must(t, os.WriteFile(filepath.Join(mainDir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0o644))

	// Zed config
	must(t, os.MkdirAll(filepath.Join(mainDir, ".zed"), 0o755))
	zedCfg := `[{"id":"zed-test","label":"Zed Test","adapter":"go","request":"launch","mode":"debug","program":"main.go"}]`
	must(t, os.WriteFile(filepath.Join(mainDir, ".zed", "debug.json"), []byte(zedCfg), 0o644))

	// VS Code config
	must(t, os.MkdirAll(filepath.Join(mainDir, ".vscode"), 0o755))
	vscodeCfg := `{"version":"0.2.0","configurations":[{"id":"vscode-test","label":"VS Code Test","type":"go","request":"launch","mode":"debug","program":"main.go"}]}`
	must(t, os.WriteFile(filepath.Join(mainDir, ".vscode", "launch.json"), []byte(vscodeCfg), 0o644))

	runGit(t, mainDir, "add", "go.mod", ".zed/debug.json", ".vscode/launch.json")
	runGit(t, mainDir, "commit", "-m", "init")

	// main repo should discover configs
	sources := scanProject(mainDir)
	if len(sources) == 0 {
		t.Fatal("expected configs in main repo, got none")
	}

	// worktree should return nil
	wtDir := filepath.Join(tmp, "worktree")
	runGit(t, mainDir, "worktree", "add", wtDir)
	sources = scanProject(wtDir)
	if len(sources) != 0 {
		t.Errorf("expected no configs from worktree, got %d", len(sources))
	}
}

func TestFindConfigsDirectSkipsWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	parent := filepath.Join(tmp, "parent")
	must(t, os.MkdirAll(parent, 0o755))

	mainDir := filepath.Join(parent, "main")
	must(t, os.MkdirAll(mainDir, 0o755))
	runGit(t, mainDir, "init")
	runGit(t, mainDir, "config", "user.email", "test@test.com")
	runGit(t, mainDir, "config", "user.name", "test")

	// VS Code config in main
	must(t, os.MkdirAll(filepath.Join(mainDir, ".vscode"), 0o755))
	vscodeCfg := `{"version":"0.2.0","configurations":[{"id":"main-test","label":"Main","type":"go","request":"launch","mode":"debug","program":"main.go"}]}`
	must(t, os.WriteFile(filepath.Join(mainDir, ".vscode", "launch.json"), []byte(vscodeCfg), 0o644))

	runGit(t, mainDir, "add", ".vscode/launch.json")
	runGit(t, mainDir, "commit", "-m", "init")

	// create worktree next to main under parent
	wtDir := filepath.Join(parent, "worktree")
	runGit(t, mainDir, "worktree", "add", wtDir)

	// findConfigsDirect only looks for .vscode/launch.json directly
	sources := findConfigsDirect(parent)
	if len(sources) == 0 {
		t.Fatal("expected at least one config from direct scan")
	}

	for _, s := range sources {
		if isGitWorktree(s.ProjectPath) {
			t.Errorf("found config from worktree at %s", s.ProjectPath)
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, out)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
