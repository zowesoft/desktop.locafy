package scaffold

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// failingInstallerStub is an installer that reports its own failure and writes
// nothing, so the service's failure path runs without a real toolchain.
func failingInstallerStub(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "failing.cmd")
		content := "@echo off\r\necho installer said no 1>&2\r\nexit /b 1\r\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "failing.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'installer said no' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestService(t *testing.T) (*Service, *sql.DB, string) {
	t.Helper()
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
	return service, database, workspace
}

func useToolStub(t *testing.T, kind string, bin string, version string) {
	t.Helper()
	restore := findToolFor
	findToolFor = func(name string) []ToolCandidate {
		if name == kind {
			return []ToolCandidate{{Path: bin, Version: version, Source: "test"}}
		}
		return nil
	}
	t.Cleanup(func() { findToolFor = restore })
}

func useDotnetStub(t *testing.T, bin string) {
	t.Helper()
	useToolStub(t, "dotnet", bin, "11.0.100")
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

// A failed project scaffold registers nothing, keeps the folder Locafy created
// for inspection and lets Discard remove exactly that folder. Django is used
// because its steps run inside the target, so Locafy is the one that created it.
func TestFailedProjectScaffoldRegistersNothingAndKeepsItsFolderUntilDiscarded(t *testing.T) {
	service, database, _ := newTestService(t)
	bin := failingInstallerStub(t, t.TempDir())
	useToolStub(t, "python", bin, "3.12.1")

	job, err := service.StartProject("Broken scaffold", "django", nil, map[string]string{"python": bin})
	if err != nil {
		t.Fatalf("StartProject: %v", err)
	}
	waitForStatus(t, service.manager, job.ID, JobFailed)

	var projects int
	if err := database.QueryRow(`SELECT count(*) FROM projects WHERE name = ?`, "Broken scaffold").Scan(&projects); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if projects != 0 {
		t.Fatalf("projects = %d, want a failed scaffold to register none", projects)
	}

	var link sql.NullString
	if err := database.QueryRow(`SELECT project_id FROM scaffold_jobs WHERE id = ?`, job.ID).Scan(&link); err != nil {
		t.Fatalf("job row: %v", err)
	}
	if link.Valid {
		t.Fatalf("job project_id = %q, want no link without a project", link.String)
	}

	_, lines, err := service.manager.Get(job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "installer said no") {
		t.Fatalf("lines = %v, want the installer's own output", lines)
	}

	if _, err := os.Stat(job.TargetPath); err != nil {
		t.Fatalf("the failed target was already removed: %v", err)
	}
	if err := service.Discard(job.ID); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := os.Stat(job.TargetPath); !os.IsNotExist(err) {
		t.Fatalf("the discarded folder still exists: %v", err)
	}
}

// A failed application scaffold inside an existing project leaves no
// application record behind.
func TestFailedApplicationScaffoldLeavesNoRecord(t *testing.T) {
	service, database, workspace := newTestService(t)
	bin := failingInstallerStub(t, t.TempDir())
	useDotnetStub(t, bin)

	root := filepath.Join(workspace, "linked")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	project, err := projects.RegisterManagedAt(database, "Linked app", root)
	if err != nil {
		t.Fatalf("register project: %v", err)
	}

	job, err := service.Start(project.ID, "dotnet", "api", nil, map[string]string{"dotnet": bin})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForStatus(t, service.manager, job.ID, JobFailed)

	var recorded int
	if err := database.QueryRow(`SELECT count(*) FROM applications WHERE project_id = ?`, project.ID).Scan(&recorded); err != nil {
		t.Fatalf("count applications: %v", err)
	}
	if recorded != 0 {
		t.Fatalf("applications = %d, want none after a failed scaffold", recorded)
	}
}
