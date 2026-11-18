//go:build !windows

package scaffold

import (
	"context"
	"os/exec"
	"syscall"
)

// commandFor runs an executable with an argv, in its own process group so a
// cancel reaches the children an installer starts.
func commandFor(ctx context.Context, bin string, args []string) *exec.Cmd {
	command := exec.CommandContext(ctx, bin, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return command
}

func killTree(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
