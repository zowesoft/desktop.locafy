package db

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDataPathLayout(t *testing.T) {
	got := DataPath("HOME")
	want := filepath.Join("HOME", "locafy", "data", "locafy.db")
	if got != want {
		t.Fatalf("DataPath = %q, want %q", got, want)
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
