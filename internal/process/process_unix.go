//go:build !windows

// Package process builds the commands Locafy runs and stops them together with
// everything they spawned. It is the one place that knows a platform's process
// rules, shared by scaffold installs and application supervision.
package process

import (
	"context"
	"os/exec"
	"syscall"
)

// CommandFor runs an executable with an argv, in its own process group so a
// stop reaches the children a server starts.
func CommandFor(ctx context.Context, bin string, args []string) *exec.Cmd {
	command := exec.CommandContext(ctx, bin, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return command
}

// KillTree stops a process and everything in its group.
func KillTree(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
