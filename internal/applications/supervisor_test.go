package applications

import (
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// script writes a small executable for a test: it prints a line and then either
// exits with the given code or stays alive for a few seconds.
func script(t *testing.T, dir string, name string, exitCode int, wait bool) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".cmd")
		var content strings.Builder
		content.WriteString("@echo off\r\necho stub ready\r\n")
		if wait {
			// ping is the portable way to wait without a console timeout.
			content.WriteString("ping -n 6 127.0.0.1 >nul\r\n")
		}
		if exitCode != 0 {
			content.WriteString("echo stub refusing 1>&2\r\n")
		}
		content.WriteString("exit /b " + itoa(exitCode) + "\r\n")
		if err := os.WriteFile(path, []byte(content.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	path := filepath.Join(dir, name+".sh")
	var content strings.Builder
	content.WriteString("#!/bin/sh\necho stub ready\n")
	if wait {
		content.WriteString("sleep 5\n")
	}
	if exitCode != 0 {
		content.WriteString("echo stub refusing 1>&2\n")
	}
	content.WriteString("exit " + itoa(exitCode) + "\n")
	if err := os.WriteFile(path, []byte(content.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

// applicationWithCommand creates a project, a folder and an application that
// runs the given command in it.
func applicationWithCommand(t *testing.T, database *sql.DB, command string) (Application, string) {
	t.Helper()

	project := newProject(t, database)
	makeFolder(t, project, "web")
	app, err := Create(database, project.ID, CreateInput{
		Name:         "API",
		Directory:    "web",
		StartCommand: command,
	})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	return app, filepath.Join(project.RootPath, "web")
}

// quoteCommand makes a path safe to store as a start command.
func quoteCommand(path string) string {
	return `"` + path + `"`
}

type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch name {
	case EventOutput:
		if line, ok := data.(map[string]any)["line"].(string); ok {
			r.events = append(r.events, line)
		}
	case EventState:
		if status, ok := data.(map[string]any)["status"].(string); ok {
			r.events = append(r.events, "state:"+status)
		}
	case EventPorts:
		if ports, ok := data.(map[string]any)["ports"].([]int); ok {
			values := make([]string, 0, len(ports))
			for _, port := range ports {
				values = append(values, strconv.Itoa(port))
			}
			r.events = append(r.events, "ports:"+strings.Join(values, ","))
		}
	case EventHealth:
		if health, ok := data.(map[string]any)["health"].(string); ok {
			r.events = append(r.events, "health:"+health)
		}
	}
}

func (r *recorder) joined() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.events, "\n")
}

func waitForStatus(t *testing.T, database *sql.DB, id string, want string) Application {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for {
		app, err := Get(database, id)
		if err != nil {
			t.Fatalf("get application: %v", err)
		}
		if app.Status == want {
			return app
		}
		if time.Now().After(deadline) {
			t.Fatalf("application stayed %q (%s), want %s", app.Status, app.Error, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestStartStreamsOutputAndRecordsTheExitCode(t *testing.T) {
	database := openDatabase(t)
	bin := script(t, t.TempDir(), "quick", 0, false)
	app, _ := applicationWithCommand(t, database, quoteCommand(bin))

	events := &recorder{}
	supervisor := NewSupervisor(database, events.emit)

	started, err := supervisor.Start(app.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if started.ID != app.ID {
		t.Fatalf("started = %+v, want the application that was started", started)
	}

	finished := waitForStatus(t, database, app.ID, StatusStopped)
	if finished.ExitCode == nil || *finished.ExitCode != 0 {
		t.Fatalf("exit code = %v, want 0", finished.ExitCode)
	}
	if finished.StartedAt == "" {
		t.Fatal("started_at was not recorded")
	}
	if !strings.Contains(events.joined(), "stub ready") {
		t.Fatalf("events = %q, want the process's own output", events.joined())
	}
	if !strings.Contains(events.joined(), "state:running") {
		t.Fatalf("events = %q, want the running transition", events.joined())
	}
}

func TestStartRefusesASecondCopyAndStopKillsTheTree(t *testing.T) {
	database := openDatabase(t)
	bin := script(t, t.TempDir(), "server", 0, true)
	app, _ := applicationWithCommand(t, database, quoteCommand(bin))

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitForStatus(t, database, app.ID, StatusRunning)

	if _, err := supervisor.Start(app.ID); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second start: err = %v, want the already-running refusal", err)
	}

	if err := supervisor.Stop(app.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// The first read after Stop must already be terminal: a project restart
	// reads the record straight away and would skip an application that still
	// says it is stopping.
	stopped, err := Get(database, app.ID)
	if err != nil {
		t.Fatalf("get after stop: %v", err)
	}
	if stopped.Status != StatusStopped {
		t.Fatalf("status right after stop = %q, want %q", stopped.Status, StatusStopped)
	}
	if stopped.Error != "" {
		t.Fatalf("error = %q, want a requested stop to leave no failure", stopped.Error)
	}
	if stopped.ExitCode != nil {
		t.Fatalf("exit code = %v, want a requested stop to leave none", stopped.ExitCode)
	}
	if err := supervisor.Stop(app.ID); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("second stop: err = %v, want the not-running refusal", err)
	}
}

func TestFailingProcessRecordsTheErrorAndItsExitCode(t *testing.T) {
	database := openDatabase(t)
	bin := script(t, t.TempDir(), "failing", 3, false)
	app, _ := applicationWithCommand(t, database, quoteCommand(bin))

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatalf("start: %v", err)
	}

	failed := waitForStatus(t, database, app.ID, StatusError)
	if failed.ExitCode == nil || *failed.ExitCode != 3 {
		t.Fatalf("exit code = %v, want 3", failed.ExitCode)
	}
	if !strings.Contains(failed.Error, "3") {
		t.Fatalf("error = %q, want the exit code explained", failed.Error)
	}
}

func TestStartExplainsWhyItCannotRun(t *testing.T) {
	database := openDatabase(t)
	supervisor := NewSupervisor(database, nil)

	empty, _ := applicationWithCommand(t, database, "")
	if _, err := supervisor.Start(empty.ID); !errors.Is(err, ErrNoStartCommand) {
		t.Fatalf("empty command: err = %v, want the missing-command refusal", err)
	}
	if failed := waitForStatus(t, database, empty.ID, StatusError); failed.Error == "" {
		t.Fatal("a refused start left no reason on the record")
	}

	missing, folder := applicationWithCommand(t, database, quoteCommand(filepath.Join(t.TempDir(), "gone.cmd")))
	if err := os.RemoveAll(folder); err != nil {
		t.Fatal(err)
	}
	if _, err := supervisor.Start(missing.ID); !errors.Is(err, ErrDirectoryMissing) {
		t.Fatalf("missing folder: err = %v, want the missing-directory refusal", err)
	}

	unknown, _ := applicationWithCommand(t, database, "locafy-tool-that-does-not-exist")
	if _, err := supervisor.Start(unknown.ID); !errors.Is(err, ErrExecutableNotFound) {
		t.Fatalf("unknown program: err = %v, want the executable-not-found refusal", err)
	}
}

func TestSplitCommandKeepsQuotedArguments(t *testing.T) {
	args, err := splitCommand(`"C:\Program Files\node.exe" run dev --host "127.0.0.1"`)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	want := []string{`C:\Program Files\node.exe`, "run", "dev", "--host", "127.0.0.1"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for index := range want {
		if args[index] != want[index] {
			t.Fatalf("args = %v, want %v", args, want)
		}
	}

	for _, broken := range []string{"", "   ", `npm "unclosed`} {
		if _, err := splitCommand(broken); err == nil {
			t.Fatalf("splitCommand(%q) accepted an invalid command", broken)
		}
	}
}

func TestSupervisorReconcilesStaleRunningRecords(t *testing.T) {
	database := openDatabase(t)
	app, _ := applicationWithCommand(t, database, "npm run dev")

	if _, err := database.Exec(`UPDATE applications SET status = ?, exit_code = 7 WHERE id = ?`,
		StatusRunning, app.ID); err != nil {
		t.Fatal(err)
	}

	NewSupervisor(database, nil)

	reconciled, err := Get(database, app.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if reconciled.Status != StatusStopped {
		t.Fatalf("status = %q, want a stale run corrected to stopped", reconciled.Status)
	}
	if reconciled.ExitCode != nil {
		t.Fatalf("exit code = %v, want it cleared", reconciled.ExitCode)
	}
	if reconciled.Error != staleRunMessage {
		t.Fatalf("error = %q, want the restart explained", reconciled.Error)
	}
}

// deadPID starts a process, reaps it and returns its pid: a pid that names no
// running process, which is what a late exit record leaves behind.
func deadPID(t *testing.T) int {
	t.Helper()

	command := exec.Command("cmd.exe", "/c", "exit 0")
	if runtime.GOOS != "windows" {
		command = exec.Command("sh", "-c", "exit 0")
	}
	if err := command.Start(); err != nil {
		t.Fatalf("start a short-lived process: %v", err)
	}
	pid := command.Process.Pid
	if err := command.Wait(); err != nil {
		t.Fatalf("reap the short-lived process: %v", err)
	}
	return pid
}

func TestStopDecidesFromTheProcessExitRatherThanItsRecord(t *testing.T) {
	database := openDatabase(t)
	app, _ := applicationWithCommand(t, database, quoteCommand(script(t, t.TempDir(), "live", 0, true)))

	supervisor := NewSupervisor(database, nil)
	t.Cleanup(supervisor.StopAll)

	previousStop := stopTimeout
	stopTimeout = 5 * time.Second
	t.Cleanup(func() { stopTimeout = previousStop })

	// The process is already gone; only the exit record is outstanding, which is
	// the state a loaded machine leaves while a child holds the output pipes.
	live := &run{done: make(chan struct{}), pid: deadPID(t), name: app.Name}
	supervisor.mu.Lock()
	supervisor.runs[app.ID] = live
	supervisor.mu.Unlock()

	start := time.Now()
	if err := supervisor.Stop(app.ID); err != nil {
		t.Fatalf("stop: %v, want a process that has exited treated as stopped", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("stop took %s, want it decided from the process rather than the timeout", elapsed)
	}
	select {
	case <-live.done:
	default:
		t.Fatal("stop released nobody: the run's waiters are still blocked")
	}
	if stopped, err := Get(database, app.ID); err != nil || stopped.Status != StatusStopped {
		t.Fatalf("application = %+v (%v), want stopped", stopped, err)
	}
	if _, running := supervisor.runs[app.ID]; running {
		t.Fatal("the run is still registered after a stop that saw the process leave")
	}
}

func TestStopStillFailsWhenTheProcessIsAliveAtTheDeadline(t *testing.T) {
	database := openDatabase(t)
	app, _ := applicationWithCommand(t, database, quoteCommand(script(t, t.TempDir(), "live", 0, true)))

	supervisor := NewSupervisor(database, nil)

	previousStop, previousAlive := stopTimeout, processAlive
	stopTimeout = 250 * time.Millisecond
	processAlive = func(int) bool { return true }
	t.Cleanup(func() {
		stopTimeout, processAlive = previousStop, previousAlive
		supervisor.mu.Lock()
		delete(supervisor.runs, app.ID)
		supervisor.mu.Unlock()
	})

	live := &run{done: make(chan struct{}), pid: 424242, name: app.Name}
	supervisor.mu.Lock()
	supervisor.runs[app.ID] = live
	supervisor.mu.Unlock()

	err := supervisor.Stop(app.ID)
	if err == nil || !strings.Contains(err.Error(), "did not stop") {
		t.Fatalf("stop = %v, want a refusal while the process is still there", err)
	}
	select {
	case <-live.done:
		t.Fatal("a process that never stopped released its waiters")
	default:
	}
}
