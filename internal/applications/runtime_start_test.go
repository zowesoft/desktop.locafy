package applications

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// machineScript writes a program that answers with its own name, so a test can
// tell which PATH copy ran. It returns the directory holding it.
func machineScript(t *testing.T, dir string, name string, word string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".cmd")
		content := "@echo off\r\necho " + word + "\r\nexit /b 0\r\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\necho " + word + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestStartResolvesTheAssignedRuntimeBeforeTheMachinePATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	}
	name := "stubtool"
	runtimeDir := machineScript(t, t.TempDir(), name, "ASSIGNED-RUNTIME")
	fallbackDir := machineScript(t, t.TempDir(), name, "MACHINE-DEFAULT")
	t.Setenv("PATH", fallbackDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	database := openDatabase(t)
	app, _ := applicationWithCommand(t, database, name)

	// Without an assignment the machine PATH decides, and it answers with the
	// fallback.
	events := &recorder{}
	supervisor := NewSupervisor(database, events.emit)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatalf("start without an assignment: %v", err)
	}
	waitForStatus(t, database, app.ID, StatusStopped)
	if !strings.Contains(events.joined(), "MACHINE-DEFAULT") {
		t.Fatalf("events = %q, want the machine PATH copy", events.joined())
	}

	// With an assignment the runtime copy answers instead, and the children it
	// starts resolve there too.
	id := "python@" + filepath.Join(runtimeDir, name+".cmd")
	if runtime.GOOS != "windows" {
		id = "python@" + filepath.Join(runtimeDir, name)
	}
	if err := SetRuntime(database, app.ID, id); err != nil {
		t.Fatalf("assign runtime: %v", err)
	}

	events = &recorder{}
	supervisor = NewSupervisor(database, events.emit)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatalf("start with an assignment: %v", err)
	}
	waitForStatus(t, database, app.ID, StatusStopped)
	if !strings.Contains(events.joined(), "ASSIGNED-RUNTIME") {
		t.Fatalf("events = %q, want the assigned copy", events.joined())
	}
}

func TestStartRefusesAnAssignedRuntimeThatIsGone(t *testing.T) {
	database := openDatabase(t)
	bin := script(t, t.TempDir(), "server", 0, true)
	app, _ := applicationWithCommand(t, database, quoteCommand(bin))

	id := stubRuntime(t, "python", "stubtool.cmd")
	if runtime.GOOS != "windows" {
		id = stubRuntime(t, "python", "stubtool")
	}
	if err := SetRuntime(database, app.ID, id); err != nil {
		t.Fatalf("assign runtime: %v", err)
	}
	if err := os.Remove(strings.TrimPrefix(id, "python@")); err != nil {
		t.Fatalf("remove the stub: %v", err)
	}

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(app.ID); err == nil {
		t.Fatal("start succeeded for a runtime that is not installed any more")
	}

	stopped := waitForStatus(t, database, app.ID, StatusError)
	if !strings.Contains(stopped.Error, "not available") {
		t.Errorf("error = %q, want the unavailable runtime explanation", stopped.Error)
	}
}
