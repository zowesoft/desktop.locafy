package applications

import (
	"errors"
	"testing"
)

func TestUpdateRewritesTheEditableFields(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "web")
	makeFolder(t, project, "api")
	app := addApplication(t, database, project.ID, "web")

	updated, err := Update(database, app.ID, UpdateInput{
		Name:         "Storefront",
		Directory:    "api",
		Language:     "TypeScript",
		Framework:    "Next.js",
		Marker:       "package.json",
		StartCommand: "npm run dev",
		StopCommand:  "npm run stop",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Storefront" || updated.Directory != "api" || updated.StartCommand != "npm run dev" {
		t.Errorf("updated = %+v, want the new fields", updated)
	}

	stored, err := Get(database, app.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Framework != "Next.js" || stored.Marker != "package.json" || stored.StopCommand != "npm run stop" {
		t.Errorf("stored = %+v, want every field persisted", stored)
	}
}

func TestUpdateKeepsEveryEditInsideTheProject(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "web")
	app := addApplication(t, database, project.ID, "web")

	// A directory outside the project stays refused, exactly as at creation.
	if _, err := Update(database, app.ID, UpdateInput{Name: "X", Directory: `C:\elsewhere`}); !errors.Is(err, ErrDirectoryOutsideProject) {
		t.Errorf("outside err = %v, want the outside refusal", err)
	}
	// A directory that does not exist is refused too.
	if _, err := Update(database, app.ID, UpdateInput{Name: "X", Directory: "nope"}); !errors.Is(err, ErrDirectoryMissing) {
		t.Errorf("missing err = %v, want the missing refusal", err)
	}
	// An empty name is refused.
	if _, err := Update(database, app.ID, UpdateInput{Name: "  ", Directory: "web"}); !errors.Is(err, ErrNameRequired) {
		t.Errorf("name err = %v, want the name refusal", err)
	}
}

func TestUpdateRefusesADirectoryAnotherApplicationUses(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "web")
	makeFolder(t, project, "api")
	web := addApplication(t, database, project.ID, "web")
	addApplication(t, database, project.ID, "api")

	if _, err := Update(database, web.ID, UpdateInput{Name: "Web", Directory: "api"}); !errors.Is(err, ErrDirectoryInUse) {
		t.Errorf("err = %v, want the in-use refusal", err)
	}
}

func TestUpdateRefusesARunningApplication(t *testing.T) {
	database := openDatabase(t)
	bin := script(t, t.TempDir(), "server", 0, true)
	app, _ := applicationWithCommand(t, database, quoteCommand(bin))

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForStatus(t, database, app.ID, StatusRunning)

	if _, err := Update(database, app.ID, UpdateInput{Name: "Renamed", Directory: "web"}); !errors.Is(err, ErrApplicationBusy) {
		t.Errorf("err = %v, want the busy refusal", err)
	}

	if err := supervisor.Stop(app.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	waitForStatus(t, database, app.ID, StatusStopped)

	// Once stopped the same edit goes through.
	updated, err := Update(database, app.ID, UpdateInput{Name: "Renamed", Directory: "web"})
	if err != nil {
		t.Fatalf("update after stop: %v", err)
	}
	if updated.Name != "Renamed" {
		t.Errorf("name = %q, want Renamed", updated.Name)
	}
}
