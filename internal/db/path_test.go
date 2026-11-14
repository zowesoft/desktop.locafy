package db

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDataPathLayout(t *testing.T) {
	got := DataPath("HOME")
	want := filepath.Join("HOME", AppDirName(), "data", "locafy.db")
	if got != want {
		t.Fatalf("DataPath = %q, want %q", got, want)
	}
}

func TestAppDirNameFollowsPlatform(t *testing.T) {
	name := AppDirName()
	if runtime.GOOS == "linux" {
		if name != "locafy" {
			t.Fatalf("AppDirName on Linux = %q, want the lowercase XDG name", name)
		}
		return
	}
	if name != "Locafy" {
		t.Fatalf("AppDirName = %q, want the branded directory name", name)
	}
}

func TestDataHomePerPlatform(t *testing.T) {
	home, err := DataHome()
	if err != nil {
		t.Fatalf("DataHome: %v", err)
	}
	if !filepath.IsAbs(home) {
		t.Fatalf("DataHome = %q, want an absolute path", home)
	}

	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(strings.ToLower(home), `appdata\roaming`) {
			t.Fatalf("DataHome on Windows = %q, want the roaming application data directory", home)
		}
	case "darwin":
		if !strings.Contains(home, "Application Support") {
			t.Fatalf("DataHome on macOS = %q, want the Application Support directory", home)
		}
	default:
		if strings.Contains(strings.ToLower(home), filepath.Join(".config", "")) {
			t.Fatalf("DataHome on Linux = %q, want the XDG data directory", home)
		}
	}
}

func TestDefaultPathCreatesDirectory(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	home, err := DataHome()
	if err != nil {
		t.Fatalf("DataHome: %v", err)
	}
	if want := DataPath(home); path != want {
		t.Fatalf("DefaultPath = %q, want %q", path, want)
	}
}

func TestDefaultProjectsDirIsFixedPlatformLocation(t *testing.T) {
	dir, err := DefaultProjectsDir()
	if err != nil {
		t.Fatalf("DefaultProjectsDir: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("DefaultProjectsDir = %q, want an absolute path", dir)
	}
	if filepath.Base(dir) != "projects" {
		t.Fatalf("DefaultProjectsDir = %q, want the projects directory", dir)
	}

	switch runtime.GOOS {
	case "windows":
		want := filepath.Join(os.Getenv("SystemDrive")+string(os.PathSeparator), "locafy", "projects")
		if dir != want {
			t.Fatalf("DefaultProjectsDir on Windows = %q, want %q", dir, want)
		}
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("UserHomeDir: %v", err)
		}
		if want := filepath.Join(home, "locafy", "projects"); dir != want {
			t.Fatalf("DefaultProjectsDir = %q, want %q", dir, want)
		}
	}

	if home, err := DataHome(); err == nil && strings.HasPrefix(strings.ToLower(dir), strings.ToLower(home)) {
		t.Fatalf("DefaultProjectsDir = %q must not live under the data home %q", dir, home)
	}
}
