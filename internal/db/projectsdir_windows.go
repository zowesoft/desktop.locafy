//go:build windows

package db

import (
	"os"
	"path/filepath"
)

// DefaultProjectsDir returns the default workspace root holding
// Locafy-managed projects: <SystemDrive>\locafy\projects, a visible fixed
// location instead of the roaming application-data directory. A stored
// preference overrides it when set.
func DefaultProjectsDir() (string, error) {
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	return filepath.Join(drive+string(os.PathSeparator), "locafy", "projects"), nil
}
