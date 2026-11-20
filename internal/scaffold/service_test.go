package scaffold

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/projects"
	"locafy.dev/desktop/internal/settings"
)

// installerStub behaves like a framework installer: it reads the --output
// folder the recipe passes and writes the marker the recipe verifies, so a
// project scaffold can run end to end without the real toolchain.
func installerStub(t *testing.T, dir string, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "installer.cmd")
		content := "@echo off\r\n" +
			"setlocal enabledelayedexpansion\r\n" +
			"set out=\r\n" +
			"set prev=\r\n" +
			"for %%a in (%*) do (\r\n" +
			"  if \"!prev!\"==\"--output\" set \"out=%%a\"\r\n" +
			"  set \"prev=%%a\"\r\n" +
			")\r\n" +
			"if not exist \"!out!\" mkdir \"!out!\"\r\n" +
			"echo done> \"!out!\\" + marker + "\"\r\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "installer.sh")
	content := "#!/bin/sh\n" +
		"out=\nprev=\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$prev\" = \"--output\" ]; then out=\"$a\"; fi\n" +
		"  prev=\"$a\"\n" +
		"done\n" +
		"mkdir -p \"$out\"\n" +
		"echo done > \"$out/" + marker + "\"\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// A project-mode job is created before its project exists, so the registration
// writes the link after the run; the final status write must not erase it.
func TestProjectScaffoldRegistersTheProjectAndLinksTheJob(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	workspace := t.TempDir()
	if err := settings.SetString(database, projects.WorkspaceRootKey, workspace); err != nil {
		t.Fatalf("set workspace: %v", err)
	}

	service := &Service{
		database:  database,
		catalogue: catalogue(t),
		manager:   NewManager(database, filepath.Join(t.TempDir(), "logs"), nil),
	}

	bin := installerStub(t, t.TempDir(), "Program.cs")
	restore := findToolFor
	findToolFor = func(name string) []ToolCandidate {
		if name == "dotnet" {
			return []ToolCandidate{{Path: bin, Version: "11.0.100", Source: "test"}}
		}
		return nil
	}
	t.Cleanup(func() { findToolFor = restore })

	job, err := service.StartProject("Link test", "dotnet", nil, map[string]string{"dotnet": bin})
	if err != nil {
		t.Fatalf("StartProject: %v", err)
	}
	waitForStatus(t, service.manager, job.ID, JobSucceeded)

	var projectID string
	err = database.QueryRow(`SELECT id FROM projects WHERE name = ? AND deleted_at IS NULL`, "Link test").Scan(&projectID)
	if err != nil {
		t.Fatalf("project row: %v", err)
	}

	var link sql.NullString
	if err := database.QueryRow(`SELECT project_id FROM scaffold_jobs WHERE id = ?`, job.ID).Scan(&link); err != nil {
		t.Fatalf("job row: %v", err)
	}
	if !link.Valid || link.String != projectID {
		t.Fatalf("job project_id = %v, want the registered project %s", link.String, projectID)
	}

	var detected int
	err = database.QueryRow(`SELECT count(*) FROM applications WHERE project_id = ? AND scaffold_recipe = 'dotnet'`, projectID).Scan(&detected)
	if err != nil {
		t.Fatalf("application row: %v", err)
	}
	if detected != 1 {
		t.Fatalf("detected applications = %d, want 1", detected)
	}

	var target string
	if err := database.QueryRow(`SELECT target_path FROM scaffold_jobs WHERE id = ?`, job.ID).Scan(&target); err != nil {
		t.Fatalf("job target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "Program.cs")); err != nil {
		t.Fatalf("scaffolded marker: %v", err)
	}
}
