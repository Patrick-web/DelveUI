//go:build windows

package updater

import "os/exec"

// detachCmd is a no-op on Windows. Auto-apply isn't wired for Windows yet —
// the updater falls back to opening the release page in the browser — so the
// helper only needs to exist for the package to compile.
func detachCmd(cmd *exec.Cmd) {}
