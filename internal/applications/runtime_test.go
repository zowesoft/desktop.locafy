package applications

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubRuntime writes a file the registry can name and returns its id, so a
// test assigns a runtime that exists without probing the machine.
func stubRuntime(t *testing.T, kind string, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	return kind + "@" + path
}

func TestSetRuntimePersistsAnAssignment(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "web")
	app := addApplication(t, database, project.ID, "web")

	id := stubRuntime(t, "python", "python.exe")
	if err := SetRuntime(database, app.ID, id); err != nil {
		t.Fatalf("assign runtime: %v", err)
	}

	stored, err := Get(database, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if stored.RuntimeID != id {
		t.Errorf("runtime = %q, want the assigned copy", stored.RuntimeID)
	}

	// An empty id clears the pin, so the machine's PATH order applies again.
	if err := SetRuntime(database, app.ID, ""); err != nil {
		t.Fatalf("clear runtime: %v", err)
	}
	stored, err = Get(database, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if stored.RuntimeID != "" {
		t.Errorf("runtime = %q, want empty", stored.RuntimeID)
	}
}

func TestSetRuntimeRefusesAKindThatDoesNotExist(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "web")
	app := addApplication(t, database, project.ID, "web")

	if err := SetRuntime(database, app.ID, "fortran@C:\\nowhere\\f90.exe"); err == nil || !strings.Contains(err.Error(), "not a runtime kind") {
		t.Errorf("err = %v, want the unknown-kind refusal", err)
	}
	stored, err := Get(database, app.ID)
	if err != nil {
		t.Fatalf("get application: %v", err)
	}
	if stored.RuntimeID != "" {
		t.Errorf("runtime = %q, want the refusal to store nothing", stored.RuntimeID)
	}
}

func TestSetRuntimeRefusesAPathThatIsGone(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	makeFolder(t, project, "web")
	app := addApplication(t, database, project.ID, "web")

	if err := SetRuntime(database, app.ID, "python@C:\\nowhere\\python.exe"); err == nil || !strings.Contains(err.Error(), "not installed any more") {
		t.Errorf("err = %v, want the gone-path refusal", err)
	}
}
