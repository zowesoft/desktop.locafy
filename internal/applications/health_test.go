package applications

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeHealth replaces the connection attempt, so tests can answer without a
// socket and without waiting for a real timeout.
func fakeHealth(t *testing.T, probe func(port int) error) {
	t.Helper()

	previous := healthProbe
	healthProbe = probe
	t.Cleanup(func() { healthProbe = previous })
}

func waitForHealth(t *testing.T, database *sql.DB, id string, want string) Application {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for {
		app, err := Get(database, id)
		if err != nil {
			t.Fatalf("get application: %v", err)
		}
		if app.Health == want {
			return app
		}
		if time.Now().After(deadline) {
			t.Fatalf("health = %q (%s), want %q", app.Health, app.HealthDetail, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func runningApplicationWithPort(t *testing.T, database *sql.DB, port int) (Application, *Supervisor, *recorder) {
	t.Helper()

	bin := script(t, t.TempDir(), "server", 0, true)
	app, _ := applicationWithCommand(t, database, quoteCommand(bin))

	fakePorts(t, []listener{{Port: port, PID: 4242, Process: "node"}},
		func(pid int, root int) bool { return pid == 4242 && root > 0 })

	events := &recorder{}
	supervisor := NewSupervisor(database, events.emit)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(app.ID) })
	waitForStatus(t, database, app.ID, StatusRunning)
	return app, supervisor, events
}

func TestHealthReportsThatTheApplicationAnswers(t *testing.T) {
	database := openDatabase(t)
	app, _, events := runningApplicationWithPort(t, database, 4200)

	fakeHealth(t, func(port int) error {
		if port != 4200 {
			t.Errorf("probed port = %d, want the discovered one", port)
		}
		return nil
	})

	healthy := waitForHealth(t, database, app.ID, HealthHealthy)
	if !strings.Contains(healthy.HealthDetail, "4200") {
		t.Fatalf("detail = %q, want the probed port", healthy.HealthDetail)
	}
	if healthy.HealthCheckedAt == "" {
		t.Fatal("the check time was not recorded")
	}
	if !strings.Contains(events.joined(), "health:healthy") {
		t.Fatalf("events = %q, want the health announced", events.joined())
	}
}

func TestHealthReportsAFailedConnection(t *testing.T) {
	database := openDatabase(t)
	app, _, _ := runningApplicationWithPort(t, database, 4200)

	fakeHealth(t, func(port int) error { return errors.New("connection refused") })

	failing := waitForHealth(t, database, app.ID, HealthUnhealthy)
	if !strings.Contains(failing.HealthDetail, "4200") {
		t.Fatalf("detail = %q, want the port named", failing.HealthDetail)
	}
	if failing.Status != StatusRunning {
		t.Fatalf("status = %q, want the process left running while it does not answer", failing.Status)
	}
}

func TestHealthClearsWhenTheProcessStops(t *testing.T) {
	database := openDatabase(t)
	app, supervisor, _ := runningApplicationWithPort(t, database, 4200)
	fakeHealth(t, func(port int) error { return nil })
	waitForHealth(t, database, app.ID, HealthHealthy)

	if err := supervisor.Stop(app.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	waitForStatus(t, database, app.ID, StatusStopped)

	stopped, err := Get(database, app.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stopped.Health != "" || stopped.HealthDetail != "" {
		t.Fatalf("health = %q (%q), want it cleared with the process", stopped.Health, stopped.HealthDetail)
	}
}

func TestRuntimeScanLeavesAnExitedProcessAlone(t *testing.T) {
	database := openDatabase(t)
	app, _ := applicationWithCommand(t, database, "npm run dev")

	fakePorts(t, []listener{{Port: 4200, PID: 4242}}, func(pid int, root int) bool { return true })
	fakeHealth(t, func(port int) error {
		t.Fatal("a health check ran after the process exited")
		return nil
	})

	live := &run{done: make(chan struct{})}
	close(live.done)
	NewSupervisor(database, nil).scanRuntime(app.ID, live, 1)

	untouched, err := Get(database, app.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if untouched.Port != nil || untouched.Health != "" {
		t.Fatalf("application = %+v, want no reading after the exit", untouched)
	}
}

func TestHealthStaysUnknownWithoutAPort(t *testing.T) {
	database := openDatabase(t)
	app, _ := applicationWithCommand(t, database, "npm run dev")

	fakePorts(t, nil, func(pid int, root int) bool { return false })
	fakeHealth(t, func(port int) error {
		t.Fatal("a health check ran without a discovered port")
		return nil
	})

	supervisor := NewSupervisor(database, nil)
	supervisor.scanRuntime(app.ID, &run{done: make(chan struct{})}, 1)

	unknown, err := Get(database, app.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if unknown.Health != "" {
		t.Fatalf("health = %q, want unknown while nothing is bound", unknown.Health)
	}
}
