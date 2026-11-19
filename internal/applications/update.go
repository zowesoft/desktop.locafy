package applications

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/projects"
)

// ErrApplicationBusy is returned when an edit or removal would reach a process
// that is starting, running or stopping: it would run the old command while
// the record says the new one.
var ErrApplicationBusy = errors.New("stop the application before changing it")

// UpdateInput carries the fields an operator may edit after the record exists.
// Source, scaffold facts and runtime assignment are not here: they are the
// record's history, or a separate control.
type UpdateInput struct {
	Name         string
	Directory    string
	Language     string
	Framework    string
	Marker       string
	StartCommand string
	StopCommand  string
}

// Update rewrites the fields an operator edits by hand. The directory is
// re-validated inside the project and against the other applications, and a
// running application refuses the edit, so the record never describes a
// different process than the one it started.
func Update(database *sql.DB, id string, input UpdateInput) (Application, error) {
	app, err := Get(database, id)
	if err != nil {
		return Application{}, err
	}
	switch app.Status {
	case StatusStarting, StatusRunning, StatusStopping:
		return Application{}, fmt.Errorf("%w: %s is %s", ErrApplicationBusy, app.Name, app.Status)
	}

	project, err := projects.Get(database, app.ProjectID)
	if err != nil {
		return Application{}, err
	}
	directory, absolute, err := resolveDirectory(project.RootPath, input.Directory)
	if err != nil {
		return Application{}, err
	}
	info, err := os.Stat(absolute)
	if errors.Is(err, os.ErrNotExist) {
		return Application{}, ErrDirectoryMissing
	}
	if err != nil {
		return Application{}, fmt.Errorf("inspect %q: %w", absolute, err)
	}
	if !info.IsDir() {
		return Application{}, ErrDirectoryNotDir
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Application{}, ErrNameRequired
	}
	if len([]rune(name)) > 120 {
		return Application{}, fmt.Errorf("%w: at most 120 characters", ErrNameRequired)
	}

	if _, err := database.Exec(`UPDATE applications
			SET name = ?, directory = ?, language = ?, framework = ?, marker = ?,
				start_command = ?, stop_command = ?
			WHERE id = ? AND deleted_at IS NULL`,
		name, directory, strings.TrimSpace(input.Language), strings.TrimSpace(input.Framework),
		strings.TrimSpace(input.Marker), strings.TrimSpace(input.StartCommand), strings.TrimSpace(input.StopCommand),
		id); err != nil {
		if isUniqueViolation(err) {
			return Application{}, ErrDirectoryInUse
		}
		return Application{}, fmt.Errorf("update application %q: %w", name, err)
	}
	if err := activity.Log(database, activity.ScopeProject, app.ProjectID, "Application updated", name); err != nil {
		return Application{}, err
	}
	return Get(database, id)
}
