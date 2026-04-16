//go:build windows

package session

import (
	"fmt"
	"os/exec"
	"syscall"
)

func processSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// On Windows, use taskkill to kill the process tree
	_ = exec.Command("taskkill", "/T", "/F", "/PID",
		fmt.Sprintf("%d", cmd.Process.Pid)).Run()
	_ = cmd.Process.Kill()
}
