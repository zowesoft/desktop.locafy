package applications

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/projects"
)

func openDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func newProject(t *testing.T, database *sql.DB) projects.Project {
	t.Helper()

	project, err := projects.Create(database, t.TempDir(), "Demo project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project
}

func makeFolder(t *testing.T, project projects.Project, relative string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(project.RootPath, filepath.FromSlash(relative)), 0o755); err != nil {
		t.Fatalf("create folder %q: %v", relative, err)
	}
}

func addApplication(t *testing.T, database *sql.DB, projectID string, directory string) Application {
	t.Helper()

	app, err := Create(database, projectID, CreateInput{Directory: directory})
	if err != nil {
		t.Fatalf("create application %q: %v", directory, err)
	}
	return app
}

func TestCreateSavesAConfirmedDetection(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "web")

	app, err := Create(database, project.ID, CreateInput{
		Name:         "Storefront",
		Directory:    "web",
		Language:     "TypeScript",
		Framework:    "Next.js",
		Marker:       "package.json",
		StartCommand: "npm run dev",
		Source:       SourceDetected,
	})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	if app.ProjectID != project.ID || app.Directory != "web" {
		t.Fatalf("application = %+v, want it inside the project's web folder", app)
	}
	if app.Language != "TypeScript" || app.Framework != "Next.js" || app.Marker != "package.json" {
		t.Fatalf("application = %+v, want the confirmed detection copied in", app)
	}
	if app.StartCommand != "npm run dev" || app.Source != SourceDetected || app.Status != StatusStopped {
		t.Fatalf("application = %+v, want the command, detected source and stopped state", app)
	}

	entries, err := activity.Recent(database, 10)
	if err != nil {
		t.Fatalf("recent activity: %v", err)
	}
	if len(entries) == 0 || entries[0].Action != "Application added" || entries[0].Detail != "Storefront" {
		t.Fatalf("activity = %+v, want the addition recorded with the application's name", entries)
	}
}

func TestCreateDefaultsNameAndSource(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "api")

	app := addApplication(t, database, project.ID, "api")

	if app.Name != "api" {
		t.Fatalf("name = %q, want the folder name", app.Name)
	}
	if app.Source != SourceManual {
		t.Fatalf("source = %q, want a manual record when detection did not propose it", app.Source)
	}
}

func TestCreateNamesARootApplicationAfterTheProject(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)

	app := addApplication(t, database, project.ID, ".")

	if app.Name != project.Name || app.Directory != "." {
		t.Fatalf("application = %+v, want the project's name at the root", app)
	}
}

func TestCreateRefusesADirectoryOutsideTheProject(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	outside := t.TempDir()

	for _, directory := range []string{"../elsewhere", outside} {
		if _, err := Create(database, project.ID, CreateInput{Directory: directory}); !errors.Is(err, ErrDirectoryOutsideProject) {
			t.Fatalf("directory %q: err = %v, want the outside-project refusal", directory, err)
		}
	}
}

func TestCreateRefusesAMissingDirectory(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)

	if _, err := Create(database, project.ID, CreateInput{Directory: "web"}); !errors.Is(err, ErrDirectoryMissing) {
		t.Fatalf("err = %v, want the missing-directory refusal", err)
	}
}

func TestCreateRefusesTwoApplicationsInTheSameDirectory(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "web")
	addApplication(t, database, project.ID, "web")

	if _, err := Create(database, project.ID, CreateInput{Directory: "./web"}); !errors.Is(err, ErrDirectoryInUse) {
		t.Fatalf("err = %v, want the directory-in-use refusal", err)
	}
}

func TestCreateRefusesAnUnknownProject(t *testing.T) {
	database := openDatabase(t)

	if _, err := Create(database, "missing", CreateInput{Directory: "."}); !errors.Is(err, projects.ErrNotFound) {
		t.Fatalf("err = %v, want the project-not-found refusal", err)
	}
}

func TestListIsScopedToItsProject(t *testing.T) {
	database := openDatabase(t)
	first := newProject(t, database)
	second := newProject(t, database)
	makeFolder(t, first, "api")
	makeFolder(t, second, "web")
	addApplication(t, database, first.ID, "api")
	addApplication(t, database, second.ID, "web")

	apps, err := List(database, first.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(apps) != 1 || apps[0].Directory != "api" {
		t.Fatalf("apps = %+v, want only the first project's application", apps)
	}
}

func TestRemoveKeepsTheFilesAndReleasesTheDirectory(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "web")
	app := addApplication(t, database, project.ID, "web")

	if err := Remove(database, app.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, err := Get(database, app.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want the removed application gone", err)
	}
	if _, err := os.Stat(filepath.Join(project.RootPath, "web")); err != nil {
		t.Fatalf("folder is gone after removal: %v", err)
	}

	// The path is free again, and the history still explains what happened.
	if _, err := Create(database, project.ID, CreateInput{Directory: "web"}); err != nil {
		t.Fatalf("re-adding the folder: %v", err)
	}
	entries, err := activity.Recent(database, 10)
	if err != nil {
		t.Fatalf("recent activity: %v", err)
	}
	found := false
	for _, entry := range entries {
		if entry.Action == "Application removed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("activity = %+v, want the removal recorded", entries)
	}
}
