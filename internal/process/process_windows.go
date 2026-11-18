//go:build windows

// Package process builds the commands Locafy runs and stops them together with
// everything they spawned. It is the one place that knows a platform's process
// rules, shared by scaffold installs and application supervision.
package process

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// createNoWindow keeps a tool's console hidden. Locafy runs commands from a
// windowless application, so without it each .bat shim and each probe would
// flash its own console window.
const createNoWindow = 0x08000000

// CommandFor runs an executable with an argv. Windows installers such as
// composer and the Laravel installer are .bat shims, which Go cannot execute
// directly, so only those go through cmd.exe — with a fixed argv that never
// contains a shell string.
func CommandFor(ctx context.Context, bin string, args []string) *exec.Cmd {
	var command *exec.Cmd
	switch strings.ToLower(filepath.Ext(bin)) {
	case ".bat", ".cmd":
		command = exec.CommandContext(ctx, "cmd.exe", append([]string{"/c", bin}, args...)...)
	default:
		command = exec.CommandContext(ctx, bin, args...)
	}
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return command
}

// KillTree stops a process and everything it spawned.
func KillTree(pid int) {
	command := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	_ = command.Run()
}
