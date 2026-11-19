package applications

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// fakePorts replaces the machine's listening sockets and the descent walk, so
// tests can describe an occupied port without opening one.
func fakePorts(t *testing.T, entries []listener, owned func(pid int, root int) bool) {
	t.Helper()

	previousListeners := listeningFor
	previousOwned := ownedBy
	listeningFor = func() ([]listener, error) { return entries, nil }
	ownedBy = owned
	t.Cleanup(func() {
		listeningFor = previousListeners
		ownedBy = previousOwned
	})
}

func setRecordedPort(t *testing.T, database *sql.DB, id string, port int) {
	t.Helper()
	if _, err := database.Exec(`UPDATE applications SET port = ? WHERE id = ?`, port, id); err != nil {
		t.Fatalf("record port: %v", err)
	}
}

func waitForRecordedPort(t *testing.T, database *sql.DB, id string, want int) Application {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for {
		app, err := Get(database, id)
		if err != nil {
			t.Fatalf("get application: %v", err)
		}
		if app.Port != nil && *app.Port == want {
			return app
		}
		if time.Now().After(deadline) {
			t.Fatalf("port = %v, want %d", app.Port, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestStartRecordsThePortTheApplicationListensOn(t *testing.T) {
	database := openDatabase(t)
	bin := script(t, t.TempDir(), "server", 0, true)
	app, _ := applicationWithCommand(t, database, quoteCommand(bin))

	fakePorts(t, []listener{
		{Port: 4200, PID: 4242, Process: "node"},
		{Port: 9000, PID: 9001, Process: "someone-else"},
	}, func(pid int, root int) bool { return pid == 4242 && root > 0 })

	events := &recorder{}
	supervisor := NewSupervisor(database, events.emit)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(app.ID) })

	recorded := waitForRecordedPort(t, database, app.ID, 4200)
	if recorded.Status != StatusRunning {
		t.Fatalf("status = %q, want the port recorded while running", recorded.Status)
	}
	if !strings.Contains(events.joined(), "ports:4200") {
		t.Fatalf("events = %q, want the discovered port announced", events.joined())
	}
	if strings.Contains(events.joined(), "9000") {
		t.Fatalf("events = %q, want only the application's own port", events.joined())
	}
}

func TestStartRefusesWhenTheRecordedPortIsHeld(t *testing.T) {
	database := openDatabase(t)
	bin := script(t, t.TempDir(), "server", 0, true)
	app, _ := applicationWithCommand(t, database, quoteCommand(bin))
	setRecordedPort(t, database, app.ID, 8123)

	fakePorts(t, []listener{
		{Port: 8123, PID: 777, Process: "python.exe"},
	}, func(pid int, root int) bool { return false })

	supervisor := NewSupervisor(database, nil)
	_, err := supervisor.Start(app.ID)
	if !errors.Is(err, ErrPortInUse) {
		t.Fatalf("err = %v, want the port-in-use refusal", err)
	}
	if !strings.Contains(err.Error(), "8123") || !strings.Contains(err.Error(), "python.exe") || !strings.Contains(err.Error(), "did not start") {
		t.Fatalf("err = %q, want the port, its owner and the refusal to stop it", err)
	}

	refused, getErr := Get(database, app.ID)
	if getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if refused.Status != StatusError || refused.PortConflict == "" {
		t.Fatalf("application = %+v, want a recorded port conflict", refused)
	}
	if refused.Error != "" {
		t.Fatalf("error = %q, want the conflict kept apart from an app failure", refused.Error)
	}
	if refused.ExitCode != nil {
		t.Fatalf("exit code = %v, want none for a start that never happened", refused.ExitCode)
	}

	// Nothing was spawned, so there is nothing to stop or kill.
	if out := supervisor.Output(app.ID); out != nil {
		t.Fatalf("output = %v, want no process", out)
	}
	if err := supervisor.Stop(app.ID); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("stop: err = %v, want the not-running refusal", err)
	}
}

func TestStartNamesTheLocafyApplicationHoldingThePort(t *testing.T) {
	database := openDatabase(t)
	bin := script(t, t.TempDir(), "server", 0, true)
	holder, _ := applicationWithCommand(t, database, quoteCommand(bin))
	second, _ := applicationWithCommand(t, database, quoteCommand(bin))
	setRecordedPort(t, database, second.ID, 8123)

	// The port belongs to a descendant of the application Locafy already runs.
	fakePorts(t, []listener{
		{Port: 8123, PID: 777, Process: "node"},
	}, func(pid int, root int) bool { return pid == 777 })

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(holder.ID); err != nil {
		t.Fatalf("start the port holder: %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(holder.ID) })
	waitForStatus(t, database, holder.ID, StatusRunning)

	_, err := supervisor.Start(second.ID)
	if !errors.Is(err, ErrPortInUse) {
		t.Fatalf("err = %v, want the port-in-use refusal", err)
	}
	if !strings.Contains(err.Error(), holder.Name) || !strings.Contains(err.Error(), "another Locafy application") {
		t.Fatalf("err = %q, want the holding application named", err)
	}
	if running := waitForStatus(t, database, holder.ID, StatusRunning); running.Status != StatusRunning {
		t.Fatalf("holder = %+v, want the first application left running", running)
	}
}

func TestPortsOwnedByKeepsOnlyTheApplicationTree(t *testing.T) {
	listeners := []listener{
		{Port: 5000, PID: os.Getpid()},
		{Port: 6000, PID: 999999},
	}
	ports := portsOwnedBy(listeners, os.Getpid())
	if len(ports) != 1 || ports[0] != 5000 {
		t.Fatalf("ports = %v, want only the current process's port", ports)
	}
	if !processOwnedBy(os.Getpid(), os.Getpid()) {
		t.Fatal("a process is not its own tree")
	}
	// Init is an ancestor of every process on Unix, so the unrelated root is
	// one no longer in service rather than pid 1.
	if processOwnedBy(os.Getpid(), 999999) {
		t.Fatal("an unrelated root claimed the process")
	}
}
