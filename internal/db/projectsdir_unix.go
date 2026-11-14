//go:build !windows

package db

import (
	"os"
	"path/filepath"
)

// DefaultProjectsDir returns the default workspace root holding
// Locafy-managed projects: ~/locafy/projects, a visible fixed location
// instead of the platform data directory. A stored preference overrides
// it when set.
func DefaultProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "locafy", "projects"), nil
}
