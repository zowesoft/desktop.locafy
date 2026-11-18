package dialogs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// A terminal is launched only on explicit operator action; the command
// line is built here so the launch rule can be tested without opening a
// window. No part of the path is interpreted by a shell unless the
// operator's own terminal interprets it.
type launchPlan struct {
	Bin  string
	Args []string
	Tool string
}

// validatedDir requires an absolute, existing directory. The path may
// come from a record on disk, so it is checked even though Locafy wrote
// it.
func validatedDir(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("a folder is required")
	}
	clean := filepath.Clean(trimmed)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("the folder %q is not an absolute path", trimmed)
	}
	info, err := os.Stat(clean)
	if err != nil {
		return "", fmt.Errorf("the folder %q is not available: %w", clean, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("the folder %q is not a directory", clean)
	}
	return clean, nil
}

// buildLaunchPlan picks the terminal to open. It never returns a plan
// whose arguments would need a shell to parse the folder path.
func buildLaunchPlan(goos, path string, lookPath func(string) (string, error)) (launchPlan, error) {
	switch goos {
	case "windows":
		if wt, err := lookPath("wt"); err == nil && wt != "" {
			return launchPlan{Bin: wt, Args: []string{"-d", path}, Tool: "Windows Terminal"}, nil
		}
		if strings.ContainsAny(path, "\"%") {
			return launchPlan{}, errors.New("Windows Terminal was not found and the folder path cannot be used with cmd")
		}
		return launchPlan{
			Bin: "cmd",
			Args: []string{"/c", "start", "", "cmd", "/k", "pushd", path},
			Tool: "Command Prompt",
		}, nil
	case "darwin":
		return launchPlan{Bin: "open", Args: []string{"-a", "Terminal", path}, Tool: "Terminal"}, nil
	case "linux":
		for _, candidate := range []struct {
			tool string
			args func(string) []string
		}{
			{"gnome-terminal", func(p string) []string { return []string{"--working-directory=" + p} }},
			{"konsole", func(p string) []string { return []string{"--workdir", p} }},
			{"xfce4-terminal", func(p string) []string { return []string{"--working-directory=" + p} }},
			{"kitty", func(p string) []string { return []string{"--directory=" + p} }},
			{"alacritty", func(p string) []string { return []string{"--working-directory", p} }},
			{"wezterm", func(p string) []string { return []string{"start", "--cwd", p} }},
		} {
			if bin, err := lookPath(candidate.tool); err == nil && bin != "" {
				return launchPlan{Bin: bin, Args: candidate.args(path), Tool: candidate.tool}, nil
			}
		}
		return launchPlan{}, errors.New("no supported terminal emulator was found (looked for gnome-terminal, konsole, xfce4-terminal, kitty, alacritty, wezterm)")
	default:
		return launchPlan{}, fmt.Errorf("no terminal launcher for %q", goos)
	}
}

// OpenInTerminal opens the operator's terminal at a folder. The folder
// must exist; the terminal is started detached so Locafy keeps running
// whether the operator closes it or not.
func (s *Service) OpenInTerminal(path string) (string, error) {
	abs, err := validatedDir(path)
	if err != nil {
		return "", err
	}

	plan, err := buildLaunchPlan(runtime.GOOS, abs, exec.LookPath)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(plan.Bin, plan.Args...)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("opening %s: %w", plan.Tool, err)
	}
	return "Opened " + plan.Tool, nil
}
