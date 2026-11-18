package dialogs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func found(tools map[string]string) func(string) (string, error) {
	return func(tool string) (string, error) {
		if path, ok := tools[tool]; ok {
			return path, nil
		}
		return "", errors.New("not found")
	}
}

func TestValidatedDirAcceptsARealDirectory(t *testing.T) {
	dir := t.TempDir()

	got, err := validatedDir(dir)
	if err != nil {
		t.Fatalf("validatedDir: %v", err)
	}
	if got != filepath.Clean(dir) {
		t.Fatalf("got %q, want %q", got, dir)
	}
}

func TestValidatedDirRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		path string
	}{
		{"blank", "   "},
		{"relative", "some/relative/path"},
		{"missing", filepath.Join(dir, "does-not-exist")},
		{"a file", file},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := validatedDir(test.path); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestLaunchPlanWindowsPrefersWindowsTerminal(t *testing.T) {
	plan, err := buildLaunchPlan("windows", `C:\work\app`, found(map[string]string{"wt": `C:\Users\me\AppData\Local\Microsoft\WindowsApps\wt.exe`}))
	if err != nil {
		t.Fatalf("buildLaunchPlan: %v", err)
	}
	if plan.Tool != "Windows Terminal" || plan.Args[0] != "-d" || plan.Args[1] != `C:\work\app` {
		t.Fatalf("unexpected plan %+v", plan)
	}
}

func TestLaunchPlanWindowsFallsBackToCommandPrompt(t *testing.T) {
	plan, err := buildLaunchPlan("windows", `C:\work\app`, found(nil))
	if err != nil {
		t.Fatalf("buildLaunchPlan: %v", err)
	}
	if plan.Bin != "cmd" || plan.Tool != "Command Prompt" {
		t.Fatalf("unexpected plan %+v", plan)
	}
}

func TestLaunchPlanWindowsFallbackRefusesShellHostilePaths(t *testing.T) {
	for _, path := range []string{`C:\work\100%done`, `C:\work\with"quote`} {
		if _, err := buildLaunchPlan("windows", path, found(nil)); err == nil {
			t.Fatalf("expected an error for %q", path)
		}
	}
}

func TestLaunchPlanDarwinUsesOpen(t *testing.T) {
	plan, err := buildLaunchPlan("darwin", "/Users/me/app", found(nil))
	if err != nil {
		t.Fatalf("buildLaunchPlan: %v", err)
	}
	if plan.Bin != "open" || plan.Args[0] != "-a" || plan.Args[1] != "Terminal" || plan.Args[2] != "/Users/me/app" {
		t.Fatalf("unexpected plan %+v", plan)
	}
}

func TestLaunchPlanLinuxUsesFirstAvailableEmulator(t *testing.T) {
	plan, err := buildLaunchPlan("linux", "/work/app", found(map[string]string{"konsole": "/usr/bin/konsole"}))
	if err != nil {
		t.Fatalf("buildLaunchPlan: %v", err)
	}
	if plan.Bin != "/usr/bin/konsole" || plan.Args[0] != "--workdir" || plan.Args[1] != "/work/app" {
		t.Fatalf("unexpected plan %+v", plan)
	}
}

func TestLaunchPlanLinuxReportsWhatWasTried(t *testing.T) {
	if _, err := buildLaunchPlan("linux", "/work/app", found(nil)); err == nil {
		t.Fatal("expected an error when no emulator exists")
	}
}

func TestLaunchPlanRejectsUnknownPlatforms(t *testing.T) {
	if _, err := buildLaunchPlan("plan9", "/work/app", found(nil)); err == nil {
		t.Fatal("expected an error for an unsupported platform")
	}
}
