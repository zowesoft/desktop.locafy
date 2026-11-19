// Package applications owns the applications inside a project: what runs,
// where, and which other applications have to be running first. Detection is
// advisory — it proposes an application from the files in a folder, and only a
// saved record turns that proposal into something Locafy can start.
package applications

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/projects"
)

// Where a record came from: proposed by detection and confirmed by the
// operator, or added by hand.
const (
	SourceDetected = "detected"
	SourceManual   = "manual"
)

// StatusStopped is the state a record starts in and returns to when its
// process exits cleanly.
const StatusStopped = "stopped"

var (
	ErrNameRequired            = errors.New("an application needs a name")
	ErrDirectoryRequired       = errors.New("an application needs a directory")
	ErrDirectoryOutsideProject = errors.New("the application directory must be inside the project")
	ErrDirectoryMissing        = errors.New("the application directory does not exist")
	ErrDirectoryNotDir         = errors.New("the application directory is a file, not a folder")
	ErrDirectoryInUse          = errors.New("another application in this project already uses this directory")
	ErrNotFound                = errors.New("application not found")
)

// Application is one runnable part of a project. Directory is relative to the
// project root and uses forward slashes, so records stay readable on any
// platform.
type Application struct {
	ID           string
	ProjectID    string
	Name         string
	Directory    string
	Language     string
	Framework    string
	Marker       string
	StartCommand string
	StopCommand  string
	// Status is the record's process state: stopped, starting, running,
	// stopping or error. ExitCode, Error and StartedAt describe the last run;
	// Port is the last port it listened on and PortConflict explains a start
	// that was refused because something else held that port.
	Status       string
	ExitCode     *int
	Error        string
	StartedAt    string
	Port         *int
	PortConflict string
	// Health is whether anything accepts a connection on the port:
	// healthy, unhealthy, or empty when unknown.
	Health          string
	HealthDetail    string
	HealthCheckedAt string
	// RuntimeID pins the runtime copy the start resolves first, as kind@path
	// from the registry. Empty keeps the machine's PATH order.
	RuntimeID string
	Source    string
	// ScaffoldRecipe names the framework recipe that produced the files, empty
	// when the application was detected or added by hand.
	ScaffoldRecipe  string
	ScaffoldOptions string
	CreatedAt       string
	UpdatedAt       string
	// DependsOn lists the applications of the same project that must be
	// running first.
	DependsOn []string
}

// CreateInput is what a confirmed application carries. Detection fills the
// same fields; nothing in it is executed here.
type CreateInput struct {
	Name            string
	Directory       string
	Language        string
	Framework       string
	Marker          string
	StartCommand    string
	StopCommand     string
	Source          string
	ScaffoldRecipe  string
	ScaffoldOptions string
}

// List returns a project's live applications ordered by name, with their
// dependencies attached.
func List(database *sql.DB, projectID string) ([]Application, error) {
	rows, err := database.Query(`SELECT id, project_id, name, directory, language, framework, marker,
			start_command, stop_command, status, exit_code, error, started_at, port, port_conflict,
			health, health_detail, health_checked_at, runtime_id, source, scaffold_recipe, scaffold_options, created_at, updated_at
		FROM applications WHERE project_id = ? AND deleted_at IS NULL ORDER BY name COLLATE NOCASE`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()

	var apps []Application
	for rows.Next() {
		app, err := scan(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	edges, err := dependencyEdges(database, projectID)
	if err != nil {
		return nil, err
	}
	for index := range apps {
		apps[index].DependsOn = edges[apps[index].ID]
	}
	return apps, nil
}

// Get returns one live application, or ErrNotFound.
func Get(database *sql.DB, id string) (Application, error) {
	row := database.QueryRow(`SELECT id, project_id, name, directory, language, framework, marker,
			start_command, stop_command, status, exit_code, error, started_at, port, port_conflict,
			health, health_detail, health_checked_at, runtime_id, source, scaffold_recipe, scaffold_options, created_at, updated_at
		FROM applications WHERE id = ? AND deleted_at IS NULL`, id)
	app, err := scan(row)
	if err != nil {
		return Application{}, err
	}
	edges, err := dependencyEdges(database, app.ProjectID)
	if err != nil {
		return Application{}, err
	}
	app.DependsOn = edges[app.ID]
	return app, nil
}

// Create confirms an application into the project. The directory must exist
// inside the project root, so a record can never point at files Locafy should
// not manage.
func Create(database *sql.DB, projectID string, input CreateInput) (Application, error) {
	project, err := projects.Get(database, projectID)
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
		name = defaultName(directory, project.Name)
	}
	if len([]rune(name)) > 120 {
		return Application{}, fmt.Errorf("%w: at most 120 characters", ErrNameRequired)
	}

	source := input.Source
	if source != SourceDetected {
		source = SourceManual
	}

	id := db.NewID()
	_, err = database.Exec(`INSERT INTO applications
			(id, project_id, name, directory, language, framework, marker, start_command, stop_command, source,
			 scaffold_recipe, scaffold_options)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, name, directory, strings.TrimSpace(input.Language), strings.TrimSpace(input.Framework),
		strings.TrimSpace(input.Marker), strings.TrimSpace(input.StartCommand), strings.TrimSpace(input.StopCommand), source,
		strings.TrimSpace(input.ScaffoldRecipe), strings.TrimSpace(input.ScaffoldOptions))
	if err != nil {
		if isUniqueViolation(err) {
			return Application{}, ErrDirectoryInUse
		}
		return Application{}, fmt.Errorf("create application %q: %w", name, err)
	}
	if err := activity.Log(database, activity.ScopeProject, projectID, "Application added", name); err != nil {
		return Application{}, err
	}
	return Get(database, id)
}

// Remove takes an application out of Locafy. Its files are untouched, and the
// dependency edges that pointed at it go with the record.
func Remove(database *sql.DB, id string) error {
	app, err := Get(database, id)
	if err != nil {
		return err
	}
	switch app.Status {
	case StatusStarting, StatusRunning, StatusStopping:
		return fmt.Errorf("%w: %s is %s", ErrApplicationBusy, app.Name, app.Status)
	}

	err = db.WithTx(database, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM application_dependencies
			WHERE application_id = ? OR depends_on_id = ?`, id, id); err != nil {
			return fmt.Errorf("remove application dependencies: %w", err)
		}
		result, err := tx.Exec(`UPDATE applications
			SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
			WHERE id = ? AND deleted_at IS NULL`, id)
		if err != nil {
			return fmt.Errorf("remove application %q: %w", id, err)
		}
		if affected, err := result.RowsAffected(); err == nil && affected == 0 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	return activity.Log(database, activity.ScopeProject, app.ProjectID, "Application removed", app.Name)
}

// resolveDirectory turns the requested directory into the stored relative form
// and the absolute path to inspect. A relative directory is taken from the
// project root; an absolute one is accepted only inside it.
func resolveDirectory(projectRoot string, directory string) (string, string, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "", "", ErrDirectoryRequired
	}

	absolute := directory
	if !filepath.IsAbs(directory) {
		absolute = filepath.Join(projectRoot, filepath.FromSlash(directory))
	}
	absolute = filepath.Clean(absolute)

	relative, err := filepath.Rel(projectRoot, absolute)
	if err != nil {
		return "", "", ErrDirectoryOutsideProject
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", ErrDirectoryOutsideProject
	}
	return filepath.ToSlash(relative), absolute, nil
}

// defaultName names a record after the folder it runs in, falling back to the
// project's own name when the folder is the project root.
func defaultName(directory string, projectName string) string {
	if directory == "." {
		return projectName
	}
	return filepath.Base(directory)
}

func scan(row interface{ Scan(...any) error }) (Application, error) {
	var app Application
	var exitCode, port sql.NullInt64
	err := row.Scan(&app.ID, &app.ProjectID, &app.Name, &app.Directory, &app.Language, &app.Framework,
		&app.Marker, &app.StartCommand, &app.StopCommand, &app.Status, &exitCode, &app.Error, &app.StartedAt,
		&port, &app.PortConflict, &app.Health, &app.HealthDetail, &app.HealthCheckedAt, &app.RuntimeID, &app.Source,
		&app.ScaffoldRecipe, &app.ScaffoldOptions, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Application{}, ErrNotFound
		}
		return Application{}, fmt.Errorf("scan application: %w", err)
	}
	if exitCode.Valid {
		value := int(exitCode.Int64)
		app.ExitCode = &value
	}
	if port.Valid {
		value := int(port.Int64)
		app.Port = &value
	}
	return app, nil
}

// dependencyEdges maps every live application of a project to what it depends
// on.
func dependencyEdges(database *sql.DB, projectID string) (map[string][]string, error) {
	rows, err := database.Query(`SELECT dependency.application_id, dependency.depends_on_id
		FROM application_dependencies AS dependency
		JOIN applications AS app ON app.id = dependency.application_id
		WHERE app.project_id = ? AND app.deleted_at IS NULL
		ORDER BY dependency.application_id, dependency.depends_on_id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list application dependencies: %w", err)
	}
	defer rows.Close()

	edges := make(map[string][]string)
	for rows.Next() {
		var applicationID, dependsOnID string
		if err := rows.Scan(&applicationID, &dependsOnID); err != nil {
			return nil, fmt.Errorf("scan application dependency: %w", err)
		}
		edges[applicationID] = append(edges[applicationID], dependsOnID)
	}
	return edges, rows.Err()
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
