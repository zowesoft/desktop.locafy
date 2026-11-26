package services

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/network"
)

// testCertificateDirectory is where TestMain points the certificate environment.
// No test may create a certificate authority in the user's data directory.
var testCertificateDirectory string

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "locafy-certificates")
	if err != nil {
		panic("create temporary certificate directory: " + err.Error())
	}
	testCertificateDirectory = directory

	previous := certificateEnvironment
	certificateEnvironment = func(env []string) []string {
		return network.CertificateEnvironmentAt(env, directory)
	}

	code := m.Run()

	certificateEnvironment = previous
	_ = os.RemoveAll(directory)
	os.Exit(code)
}

// setup gives each test a database with one project, a data root of its own and
// a stub engine, so no test needs PostgreSQL installed.
func setup(t *testing.T) (*sql.DB, string, string) {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	projectID := db.NewID()
	_, err = database.Exec(`INSERT INTO projects (id, name, root_path, kind) VALUES (?, ?, ?, 'managed')`,
		projectID, "Etugora", t.TempDir())
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	root := filepath.Join(t.TempDir(), "services")
	useStubEngine(t)
	return database, projectID, root
}

// useStubEngine replaces discovery, the port bind check and the health probe so
// a lifecycle test never touches a real engine or socket.
func useStubEngine(t *testing.T) binaries {
	t.Helper()

	programs := stubEngine(t)
	previousFind := findPostgres
	previousMySQL := findMySQL
	previousBind := bindProbe
	previousProbe := healthProbe
	previousInterval := healthInterval
	previousStop := stopTimeout

	findPostgres = func() (binaries, error) { return programs, nil }
	// MySQL stays out of the way unless a test describes it, so a developer's
	// installed copy cannot change what the catalogue tests see.
	findMySQL = func() (binaries, error) { return binaries{}, errors.New("not installed") }
	bindProbe = func(int) error { return nil }
	healthProbe = func(int) error { return nil }
	healthInterval = 30 * time.Millisecond
	stopTimeout = 2 * time.Second

	t.Cleanup(func() {
		findPostgres = previousFind
		findMySQL = previousMySQL
		bindProbe = previousBind
		healthProbe = previousProbe
		healthInterval = previousInterval
		stopTimeout = previousStop
	})
	return programs
}

// stubEngine writes the three programs the PostgreSQL lifecycle calls. initdb
// creates a cluster marker, postgres keeps a heartbeat file growing until it is
// killed, and pg_ctl records that a graceful stop was attempted.
func stubEngine(t *testing.T) binaries {
	t.Helper()
	dir := t.TempDir()
	heartbeat := filepath.Join(dir, "heartbeat.txt")
	stopMarker := filepath.Join(dir, "stop-invoked.txt")

	initdb := filepath.Join(dir, program("initdb"))
	postgres := filepath.Join(dir, program("postgres"))
	pgctl := filepath.Join(dir, program("pg_ctl"))
	createdb := filepath.Join(dir, program("createdb"))
	createdMarker := filepath.Join(dir, "database-created.txt")
	// The stub engine records the certificate variable it was started with, so a
	// test can assert what the service process actually received.
	envRecord := filepath.Join(dir, "certificate-env.txt")

	if runtime.GOOS == "windows" {
		writeScript(t, initdb,
			"@echo off\r\n"+
				"setlocal enabledelayedexpansion\r\n"+
				"set d=\r\nset prev=\r\n"+
				"for %%a in (%*) do (\r\n"+
				"  if \"!prev!\"==\"-D\" set \"d=%%a\"\r\n"+
				"  set \"prev=%%a\"\r\n"+
				")\r\n"+
				"if not exist \"!d!\" mkdir \"!d!\"\r\n"+
				"echo 17> \"!d!\\PG_VERSION\"\r\n")
		writeScript(t, postgres,
			"@echo off\r\n"+
				"setlocal enabledelayedexpansion\r\n"+
				"set d=\r\nset prev=\r\n"+
				"for %%a in (%*) do (\r\n"+
				"  if \"!prev!\"==\"-D\" set \"d=%%a\"\r\n"+
				"  set \"prev=%%a\"\r\n"+
				")\r\n"+
				"echo %NODE_EXTRA_CA_CERTS%> \""+envRecord+"\"\r\n"+
				"echo 1234> \"!d!\\postmaster.pid\"\r\n"+
				"echo started\r\n"+
				":loop\r\n"+
				"echo tick>> \""+heartbeat+"\"\r\n"+
				"ping -n 2 127.0.0.1 >nul\r\n"+
				"goto loop\r\n")
		writeScript(t, pgctl, "@echo off\r\necho stop>> \""+stopMarker+"\"\r\nexit /b 0\r\n")
		writeScript(t, createdb, "@echo off\r\necho created>> \""+createdMarker+"\"\r\nexit /b 0\r\n")
	} else {
		writeScript(t, initdb,
			"#!/bin/sh\n"+
				"d=\"\"\nprev=\"\"\n"+
				"for a in \"$@\"; do\n"+
				"  if [ \"$prev\" = \"-D\" ]; then d=\"$a\"; fi\n"+
				"  prev=\"$a\"\n"+
				"done\n"+
				"mkdir -p \"$d\"\n"+
				"echo 17 > \"$d/PG_VERSION\"\n")
		writeScript(t, postgres,
			"#!/bin/sh\n"+
				"d=\"\"\nprev=\"\"\n"+
				"for a in \"$@\"; do\n"+
				"  if [ \"$prev\" = \"-D\" ]; then d=\"$a\"; fi\n"+
				"  prev=\"$a\"\n"+
				"done\n"+
				"printf '%s' \"$NODE_EXTRA_CA_CERTS\" > \""+envRecord+"\"\n"+
				"echo 1234 > \"$d/postmaster.pid\"\n"+
				"echo started\n"+
				"while true; do echo tick >> \""+heartbeat+"\"; sleep 0.1; done\n")
		writeScript(t, pgctl, "#!/bin/sh\necho stop >> \""+stopMarker+"\"\n")
		writeScript(t, createdb, "#!/bin/sh\necho created >> \""+createdMarker+"\"\n")
	}

	return binaries{engine: EnginePostgreSQL, server: postgres, init: initdb, control: pgctl,
		client: createdb, version: "17.6", location: dir}
}

func program(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".cmd"
	}
	return name + ".sh"
}

func writeScript(t *testing.T, path string, content string) {
	t.Helper()
	mode := os.FileMode(0o644)
	if runtime.GOOS != "windows" {
		mode = 0o755
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// create attaches a PostgreSQL service with the usual defaults.
func create(t *testing.T, database *sql.DB, root string, projectID string, name string) Instance {
	t.Helper()
	instance, err := Create(database, root, projectID, CreateInput{Engine: EnginePostgreSQL, Name: name})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return instance
}

func waitFor(t *testing.T, database *sql.DB, id string, check func(Instance) bool, what string) Instance {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		instance, err := Get(database, id)
		if err != nil {
			t.Fatalf("get service: %v", err)
		}
		if check(instance) {
			return instance
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: service = %+v", what, instance)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCreateGivesEachServiceItsOwnPortAndDataDirectory(t *testing.T) {
	database, projectID, root := setup(t)
	bindProbe = func(port int) error {
		if port == 5432 {
			return errors.New("held")
		}
		return nil
	}

	first := create(t, database, root, projectID, "")
	second := create(t, database, root, projectID, "Reporting database")

	if first.Port != 5433 {
		t.Fatalf("first port = %d, want the preferred port skipped and 5433 taken", first.Port)
	}
	if second.Port != 5434 {
		t.Fatalf("second port = %d, want the next free port", second.Port)
	}
	if first.DataDirectory == second.DataDirectory {
		t.Fatalf("both services use %s, want a directory each", first.DataDirectory)
	}
	for _, instance := range []Instance{first, second} {
		if got := filepath.Dir(instance.DataDirectory); got != filepath.Join(root, instance.ID) {
			t.Fatalf("data directory %s is not keyed by the instance id under %s", instance.DataDirectory, root)
		}
		if instance.Status != StatusStopped {
			t.Fatalf("status = %q, want a new service stopped", instance.Status)
		}
		if instance.Version != "17.6" {
			t.Fatalf("version = %q, want the installed engine's", instance.Version)
		}
		if want := fmt.Sprintf("postgresql://locafy@127.0.0.1:%d/locafy", instance.Port); instance.Connection.URL != want {
			t.Fatalf("url = %q, want %q", instance.Connection.URL, want)
		}
	}
	if first.Name != "PostgreSQL" {
		t.Fatalf("name = %q, want the engine's name when none is given", first.Name)
	}

	list, err := List(database, projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %d services, want 2", len(list))
	}
}

func TestCreateRefusesAnEngineThatIsNotInstalled(t *testing.T) {
	database, projectID, root := setup(t)
	findPostgres = func() (binaries, error) { return binaries{}, errors.New("nothing here") }

	_, err := Create(database, root, projectID, CreateInput{Engine: EnginePostgreSQL})
	if !errors.Is(err, ErrEngineUnavailable) {
		t.Fatalf("err = %v, want the engine-unavailable refusal", err)
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("err = %q, want the missing engine named", err)
	}

	list, err := List(database, projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("list = %+v, want nothing recorded", list)
	}
}

func TestCreateRefusesAnEngineOutsideTheCatalogue(t *testing.T) {
	database, projectID, root := setup(t)

	_, err := Create(database, root, projectID, CreateInput{Engine: "redis"})
	if !errors.Is(err, ErrUnknownEngine) {
		t.Fatalf("err = %v, want the unknown-engine refusal", err)
	}
}

func TestCatalogueReportsWhetherTheEngineIsInstalled(t *testing.T) {
	setup(t)

	templates := Catalogue()
	if len(templates) != 2 || templates[0].Engine != EnginePostgreSQL || templates[1].Engine != EngineMySQL {
		t.Fatalf("catalogue = %+v, want PostgreSQL and MySQL", templates)
	}
	if !templates[0].Available || templates[0].Version != "17.6" || templates[0].Location == "" {
		t.Fatalf("template = %+v, want the installed engine described", templates[0])
	}
	if templates[1].Available || !strings.Contains(templates[1].Detail, "not installed") {
		t.Fatalf("template = %+v, want the absent MySQL reported with its reason", templates[1])
	}

	findPostgres = func() (binaries, error) { return binaries{}, errors.New("not found") }
	missing := Catalogue()
	if missing[0].Available {
		t.Fatalf("template = %+v, want the missing engine reported", missing[0])
	}
	if !strings.Contains(missing[0].Detail, "not installed") {
		t.Fatalf("detail = %q, want the reason", missing[0].Detail)
	}
}

func TestStartInitialisesTheDataDirectoryAndReportsHealth(t *testing.T) {
	database, projectID, root := setup(t)
	instance := create(t, database, root, projectID, "")
	supervisor := NewSupervisor(database, nil)
	t.Cleanup(supervisor.StopAll)

	if _, err := supervisor.Start(instance.ID); err != nil {
		t.Fatalf("start: %v", err)
	}

	running := waitFor(t, database, instance.ID, func(service Instance) bool {
		return service.Health == HealthHealthy
	}, "waiting for a healthy service")

	if running.Status != StatusRunning {
		t.Fatalf("status = %q, want running", running.Status)
	}
	if running.PID == nil {
		t.Fatal("pid is nil, want the supervised process recorded")
	}
	if running.StartedAt == "" {
		t.Fatal("started_at is empty")
	}
	if running.HealthDetail != fmt.Sprintf("Answering on port %d.", instance.Port) {
		t.Fatalf("health detail = %q", running.HealthDetail)
	}
	if _, err := os.Stat(filepath.Join(instance.DataDirectory, "PG_VERSION")); err != nil {
		t.Fatalf("initialised data directory: %v", err)
	}
	binaries, err := findPostgres()
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	createdMarker := filepath.Join(filepath.Dir(binaries.client), "database-created.txt")
	if _, err := os.Stat(createdMarker); err != nil {
		t.Fatalf("database marker: %v, want the project's database created once the cluster answers", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(supervisor.Output(instance.ID)) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if len(supervisor.Output(instance.ID)) == 0 {
		t.Fatal("output is empty, want the engine's own output captured")
	}
}

func TestStartReportsAMissingEngineInsteadOfFailingSilently(t *testing.T) {
	database, projectID, root := setup(t)
	instance := create(t, database, root, projectID, "")
	findPostgres = func() (binaries, error) { return binaries{}, errors.New("nothing here") }

	supervisor := NewSupervisor(database, nil)
	_, err := supervisor.Start(instance.ID)
	if !errors.Is(err, ErrEngineUnavailable) {
		t.Fatalf("err = %v, want the engine-unavailable refusal", err)
	}

	failed, err := Get(database, instance.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if failed.Status != StatusError || !strings.Contains(failed.Error, "not installed") {
		t.Fatalf("service = %+v, want the failure recorded on its own record", failed)
	}
}

func TestStartGivesTheServiceLocafysCertificate(t *testing.T) {
	database, projectID, root := setup(t)
	instance := create(t, database, root, projectID, "certified")
	supervisor := NewSupervisor(database, nil)
	t.Cleanup(supervisor.StopAll)

	binaries, err := findPostgres()
	if err != nil {
		t.Fatalf("stub engine: %v", err)
	}
	record := filepath.Join(binaries.location, "certificate-env.txt")

	if _, err := supervisor.Start(instance.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Wait for the run to settle before the test ends, so the teardown stops a
	// started service rather than one still coming up.
	waitFor(t, database, instance.ID, func(service Instance) bool {
		return service.Health == HealthHealthy
	}, "waiting for a healthy service")

	deadline := time.Now().Add(10 * time.Second)
	for {
		raw, readErr := os.ReadFile(record)
		if readErr == nil && strings.TrimSpace(string(raw)) != "" {
			received := strings.TrimSpace(string(raw))
			want := filepath.Join(testCertificateDirectory, "ca.crt")
			if received != want {
				t.Fatalf("NODE_EXTRA_CA_CERTS = %q, want %q", received, want)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the service never recorded NODE_EXTRA_CA_CERTS (read error: %v)", readErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestStopEndsTheProcessAndClearsHealth(t *testing.T) {
	database, projectID, root := setup(t)
	instance := create(t, database, root, projectID, "")
	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(instance.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, database, instance.ID, func(service Instance) bool {
		return service.Health == HealthHealthy
	}, "waiting for a healthy service")

	if err := supervisor.Stop(instance.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}

	stopped, err := Get(database, instance.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stopped.Status != StatusStopped {
		t.Fatalf("status = %q, want stopped", stopped.Status)
	}
	if stopped.PID != nil {
		t.Fatalf("pid = %v, want none after a stop", *stopped.PID)
	}
	if stopped.Health != "" || stopped.HealthDetail != "" {
		t.Fatalf("health = %q/%q, want the stale reading cleared", stopped.Health, stopped.HealthDetail)
	}
	if stopped.Error != "" {
		t.Fatalf("error = %q, want a stop that is not a failure", stopped.Error)
	}
	if err := supervisor.Stop(instance.ID); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("second stop: err = %v, want the not-running refusal", err)
	}
	// A stopped service keeps what its last run said, so the interface can still
	// show it; what it must not do is keep reading a process that is gone.
	out := strings.Join(supervisor.Output(instance.ID), "\n")
	if !strings.Contains(out, "started") {
		t.Fatalf("output = %q, want the last run's words kept after a stop", out)
	}

	binaries, err := findPostgres()
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	stopMarker := filepath.Join(filepath.Dir(binaries.control), "stop-invoked.txt")
	if _, err := os.Stat(stopMarker); err != nil {
		t.Fatalf("graceful stop marker: %v", err)
	}
}

// deadPID reaps a short-lived process and answers its pid, so a test can hold a
// process that is certainly gone.
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

func TestStopDecidesFromTheServiceExitRatherThanItsRecord(t *testing.T) {
	database, projectID, root := setup(t)
	instance := create(t, database, root, projectID, "")
	supervisor := NewSupervisor(database, nil)

	live := &liveService{done: make(chan struct{}), pid: deadPID(t), stopped: true, lines: []string{"shutting down"}}
	supervisor.mu.Lock()
	supervisor.runs[instance.ID] = live
	supervisor.mu.Unlock()

	started := time.Now()
	if err := supervisor.Stop(instance.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("stop took %s, want a decision from the exit rather than the %s timeout", elapsed, stopTimeout)
	}
	select {
	case <-live.done:
	default:
		t.Fatal("stop returned without releasing the run's waiters")
	}

	stopped, err := Get(database, instance.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stopped.Status != StatusStopped || stopped.Error != "" {
		t.Fatalf("service = %q/%q, want a clean stop", stopped.Status, stopped.Error)
	}
	supervisor.mu.Lock()
	_, running := supervisor.runs[instance.ID]
	supervisor.mu.Unlock()
	if running {
		t.Fatal("the run is still registered after a stop")
	}
	if out := strings.Join(supervisor.Output(instance.ID), "\n"); !strings.Contains(out, "shutting down") {
		t.Fatalf("output = %q, want the last run's words kept", out)
	}
}

func TestStopStillFailsWhenTheServiceProcessIsAliveAtTheDeadline(t *testing.T) {
	database, projectID, root := setup(t)
	instance := create(t, database, root, projectID, "")
	supervisor := NewSupervisor(database, nil)

	previousAlive, previousStop, previousKill := processAlive, stopTimeout, killTimeout
	processAlive = func(int) bool { return true }
	stopTimeout = 250 * time.Millisecond
	killTimeout = 250 * time.Millisecond
	t.Cleanup(func() {
		processAlive, stopTimeout, killTimeout = previousAlive, previousStop, previousKill
		supervisor.mu.Lock()
		delete(supervisor.runs, instance.ID)
		supervisor.mu.Unlock()
	})

	live := &liveService{done: make(chan struct{}), pid: 424242, stopped: true}
	supervisor.mu.Lock()
	supervisor.runs[instance.ID] = live
	supervisor.mu.Unlock()

	err := supervisor.Stop(instance.ID)
	if err == nil || !strings.Contains(err.Error(), "did not stop") {
		t.Fatalf("stop = %v, want a refusal while the process is still there", err)
	}
	select {
	case <-live.done:
		t.Fatal("a process that never stopped released its waiters")
	default:
	}
}

func TestRestartKeepsTheDataDirectoryAndPort(t *testing.T) {
	database, projectID, root := setup(t)
	instance := create(t, database, root, projectID, "")
	supervisor := NewSupervisor(database, nil)
	t.Cleanup(supervisor.StopAll)
	if _, err := supervisor.Start(instance.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, database, instance.ID, func(service Instance) bool {
		return service.Health == HealthHealthy
	}, "waiting for a healthy service")

	restarted, err := supervisor.Restart(instance.ID)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if restarted.DataDirectory != instance.DataDirectory || restarted.Port != instance.Port {
		t.Fatalf("restarted = %+v, want the same directory and port", restarted)
	}
	waitFor(t, database, instance.ID, func(service Instance) bool {
		return service.Status == StatusRunning && service.Health == HealthHealthy
	}, "waiting for the restarted service")
}

func TestRemoveKeepsTheDataDirectoryAndItsFiles(t *testing.T) {
	database, projectID, root := setup(t)
	instance := create(t, database, root, projectID, "")
	supervisor := NewSupervisor(database, nil)
	t.Cleanup(supervisor.StopAll)
	if _, err := supervisor.Start(instance.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, database, instance.ID, func(service Instance) bool {
		return service.Status == StatusRunning
	}, "waiting for a running service")

	table := filepath.Join(instance.DataDirectory, "base", "marker.txt")
	if err := os.MkdirAll(filepath.Dir(table), 0o755); err != nil {
		t.Fatalf("create data: %v", err)
	}
	if err := os.WriteFile(table, []byte("rows"), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	if err := supervisor.Stop(instance.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	removal, err := Remove(database, instance.ID)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if removal.DataDirectory != instance.DataDirectory {
		t.Fatalf("removal = %+v, want the kept data directory named", removal)
	}
	if _, err := os.Stat(table); err != nil {
		t.Fatalf("data file after removal: %v, want it kept", err)
	}
	if _, err := Get(database, instance.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want the record gone", err)
	}

	list, err := List(database, projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("list = %+v, want the removed service gone", list)
	}
}

func TestRemoveStopsARunningServiceFirst(t *testing.T) {
	database, projectID, root := setup(t)
	service := NewService(database)
	service.root = root

	instance, err := service.Create(projectID, CreateInput{Engine: EnginePostgreSQL, Name: "Cache database"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := service.Start(instance.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitFor(t, database, instance.ID, func(running Instance) bool {
		return running.Status == StatusRunning
	}, "waiting for a running service")

	if _, err := service.Remove(instance.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := Get(database, instance.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want the record gone", err)
	}
	if err := service.Stop(instance.ID); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("stop after remove: err = %v, want the process stopped by the removal", err)
	}
}

// A cluster that has just bound its port can still refuse queries while it is
// in startup recovery; the database creation must retry instead of reporting a
// failure that is really "not yet".
func TestDatabasePreparationRetriesWhileTheClusterStarts(t *testing.T) {
	database, projectID, root := setup(t)
	binaries, err := findPostgres()
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	flag := filepath.Join(filepath.Dir(binaries.client), "first-attempt.txt")
	success := filepath.Join(filepath.Dir(binaries.client), "database-created.txt")
	if err := os.Remove(success); err != nil && !os.IsNotExist(err) {
		t.Fatalf("clear marker: %v", err)
	}
	if runtime.GOOS == "windows" {
		writeScript(t, binaries.client,
			"@echo off\r\n"+
				"if exist \""+flag+"\" (\r\n"+
				"  echo created>> \""+success+"\"\r\n"+
				"  exit /b 0\r\n"+
				")\r\n"+
				"type nul> \""+flag+"\"\r\n"+
				"echo the database system is starting up 1>&2\r\n"+
				"exit /b 1\r\n")
	} else {
		writeScript(t, binaries.client,
			"#!/bin/sh\n"+
				"if [ -f \""+flag+"\" ]; then echo created >> \""+success+"\"; exit 0; fi\n"+
				"touch \""+flag+"\"\n"+
				"echo 'the database system is starting up' >&2\n"+
				"exit 1\n")
	}

	instance := create(t, database, root, projectID, "")
	supervisor := NewSupervisor(database, nil)
	t.Cleanup(supervisor.StopAll)
	if _, err := supervisor.Start(instance.ID); err != nil {
		t.Fatalf("start: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(success); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the database was never created on a retry")
		}
		time.Sleep(50 * time.Millisecond)
	}
	settled := waitFor(t, database, instance.ID, func(service Instance) bool {
		return service.Health == HealthHealthy
	}, "waiting for a healthy service")
	if settled.Error != "" {
		t.Fatalf("error = %q, want the startup refusal retried instead of recorded", settled.Error)
	}
}

// A start can fail after the engine has said why. Those last words must stay
// readable once the process is gone, and must not leak into the next run.
func TestOutputKeepsWhatAProcessSaidBeforeItExited(t *testing.T) {
	database, projectID, root := setup(t)
	binaries, err := findPostgres()
	if err != nil {
		t.Fatalf("engine: %v", err)
	}

	if runtime.GOOS == "windows" {
		writeScript(t, binaries.server,
			"@echo off\r\n"+
				"setlocal enabledelayedexpansion\r\n"+
				"set d=\r\nset prev=\r\n"+
				"for %%a in (%*) do (\r\n"+
				"  if \"!prev!\"==\"-D\" set \"d=%%a\"\r\n"+
				"  set \"prev=%%a\"\r\n"+
				")\r\n"+
				"echo 1234> \"!d!\\postmaster.pid\"\r\n"+
				"echo 2026-11-23 10:00:00.000 GMT [1234] FATAL: could not create shared memory segment 1>&2\r\n"+
				"exit /b 1\r\n")
	} else {
		writeScript(t, binaries.server,
			"#!/bin/sh\n"+
				"d=\"\"\nprev=\"\"\n"+
				"for a in \"$@\"; do\n"+
				"  if [ \"$prev\" = \"-D\" ]; then d=\"$a\"; fi\n"+
				"  prev=\"$a\"\n"+
				"done\n"+
				"echo 1234 > \"$d/postmaster.pid\"\n"+
				"echo '2026-11-23 10:00:00.000 GMT [1234] FATAL: could not create shared memory segment' >&2\n"+
				"exit 1\n")
	}

	instance := create(t, database, root, projectID, "")
	supervisor := NewSupervisor(database, nil)
	t.Cleanup(supervisor.StopAll)
	if _, err := supervisor.Start(instance.ID); err != nil {
		t.Fatalf("start: %v", err)
	}

	waitFor(t, database, instance.ID, func(service Instance) bool {
		return service.Status == StatusError
	}, "waiting for the failed start")
	said := strings.Join(supervisor.Output(instance.ID), "\n")
	if !strings.Contains(said, "could not create shared memory segment") {
		t.Fatalf("output = %q, want the process's own explanation kept after it exited", said)
	}

	if runtime.GOOS == "windows" {
		writeScript(t, binaries.server, "@echo off\r\necho started\r\nexit /b 0\r\n")
	} else {
		writeScript(t, binaries.server, "#!/bin/sh\necho started\nexit 0\n")
	}
	if _, err := supervisor.Start(instance.ID); err != nil {
		t.Fatalf("start again: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		lines := supervisor.Output(instance.ID)
		if len(lines) == 1 && strings.Contains(lines[0], "started") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("output after a restart = %q, want only the new run's line", lines)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// stubMySQL writes the programs the MySQL lifecycle calls. The initialiser
// records its argv and creates the system schema marker, the server keeps a
// heartbeat going, the client records that the database was offered and
// mysqladmin records the graceful-stop attempt.
func stubMySQL(t *testing.T) binaries {
	t.Helper()
	dir := t.TempDir()
	argsRecord := filepath.Join(dir, "init-args.txt")
	heartbeat := filepath.Join(dir, "heartbeat.txt")
	stopMarker := filepath.Join(dir, "stop-invoked.txt")
	createdMarker := filepath.Join(dir, "database-created.txt")

	mysqld := filepath.Join(dir, program("mysqld"))
	client := filepath.Join(dir, program("mysql"))
	admin := filepath.Join(dir, program("mysqladmin"))

	if runtime.GOOS == "windows" {
		writeScript(t, mysqld,
			"@echo off\r\n"+
				"setlocal enabledelayedexpansion\r\n"+
				"echo %*>> \""+argsRecord+"\"\r\n"+
				"if not \"%~1\"==\"--initialize-insecure\" goto serve\r\n"+
				"rem cmd splits --datadir=<path> into two batch arguments.\r\n"+
				"set d=%~3\r\n"+
				"if not exist \"!d!\" mkdir \"!d!\"\r\n"+
				"mkdir \"!d!\\mysql\"\r\n"+
				"exit /b 0\r\n"+
				":serve\r\n"+
				"echo started\r\n"+
				":loop\r\n"+
				"echo tick>> \""+heartbeat+"\"\r\n"+
				"ping -n 2 127.0.0.1 >nul\r\n"+
				"goto loop\r\n")
		writeScript(t, client, "@echo off\r\necho created>> \""+createdMarker+"\"\r\nexit /b 0\r\n")
		writeScript(t, admin, "@echo off\r\necho stop>> \""+stopMarker+"\"\r\nexit /b 0\r\n")
	} else {
		writeScript(t, mysqld,
			"#!/bin/sh\n"+
				"d=\"\"\ninit=\"\"\n"+
				"for a in \"$@\"; do\n"+
				"  printf '%s\n' \"$a\" >> \""+argsRecord+"\"\n"+
				"  case \"$a\" in\n"+
				"    --datadir=*) d=\"${a#--datadir=}\" ;;\n"+
				"    --initialize-insecure) init=1 ;;\n"+
				"  esac\n"+
				"done\n"+
				"if [ -n \"$init\" ]; then\n"+
				"  mkdir -p \"$d/mysql\"\n"+
				"  exit 0\n"+
				"fi\n"+
				"echo started\n"+
				"while true; do echo tick >> \""+heartbeat+"\"; sleep 0.1; done\n")
		writeScript(t, client, "#!/bin/sh\necho created >> \""+createdMarker+"\"\n")
		writeScript(t, admin, "#!/bin/sh\necho stop >> \""+stopMarker+"\"\n")
	}
	return binaries{engine: EngineMySQL, server: mysqld, init: mysqld, control: admin,
		client: client, version: "8.0.30", location: dir}
}

// createEngine attaches a service for one engine with the given name.
func createEngine(t *testing.T, database *sql.DB, root string, projectID string, engine string, name string) Instance {
	t.Helper()
	instance, err := Create(database, root, projectID, CreateInput{Engine: engine, Name: name})
	if err != nil {
		t.Fatalf("create %s service: %v", engine, err)
	}
	return instance
}

func TestCatalogueOffersMySQLWhenItIsInstalled(t *testing.T) {
	setup(t)
	programs := stubMySQL(t)
	findMySQL = func() (binaries, error) { return programs, nil }

	templates := Catalogue()
	if len(templates) != 2 {
		t.Fatalf("catalogue = %+v, want both engines", templates)
	}
	mysql := templates[1]
	if mysql.Engine != EngineMySQL || !mysql.Available {
		t.Fatalf("template = %+v, want MySQL described as installed", mysql)
	}
	if mysql.Version != "8.0.30" || mysql.Location != programs.server || mysql.DefaultPort != 3306 {
		t.Fatalf("template = %+v, want the discovered version, program and port", mysql)
	}
}

func TestCreateGivesMySQLItsOwnDefaults(t *testing.T) {
	database, projectID, root := setup(t)
	programs := stubMySQL(t)
	findMySQL = func() (binaries, error) { return programs, nil }

	instance := createEngine(t, database, root, projectID, EngineMySQL, "")
	if instance.Engine != EngineMySQL || instance.Version != "8.0.30" {
		t.Fatalf("service = %+v, want a MySQL instance with the discovered version", instance)
	}
	if instance.Port != 3306 {
		t.Fatalf("port = %d, want the engine's preferred port", instance.Port)
	}
	if instance.UserName != "root" || instance.DatabaseName != "locafy" {
		t.Fatalf("user/database = %q/%q, want root/locafy", instance.UserName, instance.DatabaseName)
	}
	want := fmt.Sprintf("mysql://root@127.0.0.1:%d/locafy", instance.Port)
	if instance.Connection.URL != want || instance.Connection.Environment != "DATABASE_URL="+want {
		t.Fatalf("connection = %+v, want the mysql URL %q", instance.Connection, want)
	}
}

func TestMySQLStartInitialisesAndCreatesTheDatabase(t *testing.T) {
	database, projectID, root := setup(t)
	programs := stubMySQL(t)
	findMySQL = func() (binaries, error) { return programs, nil }

	instance := createEngine(t, database, root, projectID, EngineMySQL, "MySQL")
	supervisor := NewSupervisor(database, nil)
	t.Cleanup(supervisor.StopAll)

	if _, err := supervisor.Start(instance.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	running := waitFor(t, database, instance.ID, func(service Instance) bool {
		return service.Health == HealthHealthy
	}, "waiting for a healthy MySQL instance")

	if running.Status != StatusRunning || running.PID == nil {
		t.Fatalf("service = %+v, want a running supervised process", running)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(programs.server), "init-args.txt"))
	if err != nil {
		t.Fatalf("read the initialiser arguments: %v", err)
	}
	if _, err := os.Stat(filepath.Join(instance.DataDirectory, "mysql")); err != nil {
		t.Fatalf("initialised data directory: %v (args: %q)", err, string(raw))
	}
	args := string(raw)
	for _, want := range []string{"--initialize-insecure", "--datadir=" + instance.DataDirectory, "--basedir="} {
		if !strings.Contains(args, want) {
			t.Fatalf("initialiser arguments = %q, want %q", args, want)
		}
	}
	if !strings.Contains(args, "--bind-address=127.0.0.1") {
		t.Fatalf("server arguments = %q, want the loopback binding", args)
	}

	created := filepath.Join(filepath.Dir(programs.client), "database-created.txt")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(created); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the project's database was never created once the port answered")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := supervisor.Stop(instance.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	stopMarker := filepath.Join(filepath.Dir(programs.control), "stop-invoked.txt")
	if _, err := os.Stat(stopMarker); err != nil {
		t.Fatalf("graceful stop marker: %v", err)
	}
	stopped, err := Get(database, instance.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stopped.Status != StatusStopped || stopped.PID != nil {
		t.Fatalf("service = %+v, want it stopped with no pid", stopped)
	}
}

// MySQL answers @@datadir with doubled separators and a trailing one, and can
// differ in case; the comparison has to see through that. A different server's
// data directory must never be accepted as this instance's.
func TestSameDirectoryReadsTheEnginesOwnAnswer(t *testing.T) {
	want := `C:\Users\me\AppData\Roaming\Locafy\services\id\data`
	cases := []struct {
		reported string
		same     bool
	}{
		{`C:\Users\me\AppData\Roaming\Locafy\services\id\data\`, true},
		{`c:/users/me/appdata/roaming/locafy/services/id/data/`, true},
		{`C:\laragon\data\mysql-8\`, false},
		{`C:\Users\me\AppData\Roaming\Locafy\services\other\data\`, false},
		{"", false},
	}
	for _, test := range cases {
		if got := sameDirectory(test.reported, want); got != test.same {
			t.Errorf("sameDirectory(%q) = %v, want %v", test.reported, got, test.same)
		}
	}
}
