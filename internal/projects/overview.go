package projects

import (
	"fmt"
	"os/exec"
	"runtime"

	"locafy.dev/desktop/internal/activity"
)

// Activity returns recent recorded actions for one project, newest first, for
// the overview's activity feed.
func (s *Service) Activity(id string, limit int) ([]activity.Entry, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if _, err := Get(s.database, id); err != nil {
		return nil, err
	}
	return activity.RecentFor(s.database, activity.ScopeProject, id, limit)
}

// OpenFolder reveals a project's directory in the OS file manager.
func (s *Service) OpenFolder(id string) error {
	project, err := Get(s.database, id)
	if err != nil {
		return err
	}
	if err := openProjectFolder(project.RootPath); err != nil {
		return fmt.Errorf("open project folder %q: %w", project.RootPath, err)
	}
	return nil
}

// openProjectFolder is a seam for tests: the real implementation hands the path
// to the OS, the only way to reveal a folder from inside a webview.
var openProjectFolder = func(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
