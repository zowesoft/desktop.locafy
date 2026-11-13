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

// DataPath returns the database file path under a data home directory.
func DataPath(dataHome string) string {
	return filepath.Join(dataHome, "locafy", "data", "locafy.db")
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
