package db

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/adrg/xdg"
)

// DataHome returns the platform data directory holding Locafy state:
// the roaming application-data directory on Windows and macOS, the XDG
// data directory elsewhere, matching the recorded path policy.
func DataHome() (string, error) {
	switch runtime.GOOS {
	case "windows", "darwin":
		return os.UserConfigDir()
	default:
		return xdg.DataHome, nil
	}
}

// AppDirName returns the directory Locafy owns inside the platform data
// home. It is branded on Windows and macOS and lowercase on Linux, where
// XDG expects directory names to be lowercase.
func AppDirName() string {
	if runtime.GOOS == "linux" {
		return "locafy"
	}
	return "Locafy"
}

// DataPath returns the database file path under a data home directory.
func DataPath(dataHome string) string {
	return filepath.Join(dataHome, AppDirName(), "data", "locafy.db")
}

// DefaultPath resolves the database path for this machine and creates the
// containing directory when missing.
func DefaultPath() (string, error) {
	home, err := DataHome()
	if err != nil {
		return "", err
	}
	path := DataPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, nil
}
