//go:build windows

package scaffold

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// createNoWindow keeps a tool's console hidden. Locafy runs installers from a
// windowless application, so without it each .bat shim and each version probe
// would flash its own console window.
const createNoWindow = 0x08000000

// commandFor runs an executable with an argv. Windows installers such as
// composer and the Laravel installer are .bat shims, which Go cannot execute
// directly, so only those go through cmd.exe — with a fixed argv that never
// contains a shell string.
func commandFor(ctx context.Context, bin string, args []string) *exec.Cmd {
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

// killTree stops a scaffold and everything it spawned.
func killTree(pid int) {
	command := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	_ = command.Run()
}
