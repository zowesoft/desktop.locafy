package services

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"locafy.dev/desktop/internal/db"
)

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
func useStubEngine(t *testing.T) postgresBinaries {
	t.Helper()

	binaries := stubEngine(t)
	previousFind := findPostgres
	previousBind := bindProbe
	previousProbe := healthProbe
	previousInterval := healthInterval
	previousStop := stopTimeout

	findPostgres = func() (postgresBinaries, error) { return binaries, nil }
	bindProbe = func(int) error { return nil }
	healthProbe = func(int) error { return nil }
	healthInterval = 30 * time.Millisecond
	stopTimeout = 2 * time.Second

	t.Cleanup(func() {
		findPostgres = previousFind
		bindProbe = previousBind
		healthProbe = previousProbe
		healthInterval = previousInterval
		stopTimeout = previousStop
	})
	return binaries
}

// stubEngine writes the three programs the PostgreSQL lifecycle calls. initdb
// creates a cluster marker, postgres keeps a heartbeat file growing until it is
// killed, and pg_ctl records that a graceful stop was attempted.
func stubEngine(t *testing.T) postgresBinaries {
	t.Helper()
	dir := t.TempDir()
	heartbeat := filepath.Join(dir, "heartbeat.txt")
	stopMarker := filepath.Join(dir, "stop-invoked.txt")

	initdb := filepath.Join(dir, program("initdb"))
	postgres := filepath.Join(dir, program("postgres"))
	pgctl := filepath.Join(dir, program("pg_ctl"))
	createdb := filepath.Join(dir, program("createdb"))
	createdMarker := filepath.Join(dir, "database-created.txt")

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
				"echo 1234 > \"$d/postmaster.pid\"\n"+
				"echo started\n"+
				"while true; do echo tick >> \""+heartbeat+"\"; sleep 0.1; done\n")
		writeScript(t, pgctl, "#!/bin/sh\necho stop >> \""+stopMarker+"\"\n")
		writeScript(t, createdb, "#!/bin/sh\necho created >> \""+createdMarker+"\"\n")
	}

	return postgresBinaries{postgres: postgres, initdb: initdb, pgctl: pgctl, createdb: createdb, version: "17.6", location: dir}
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
	findPostgres = func() (postgresBinaries, error) { return postgresBinaries{}, errors.New("nothing here") }

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
	if len(templates) != 1 || templates[0].Engine != EnginePostgreSQL {
		t.Fatalf("catalogue = %+v, want PostgreSQL", templates)
	}
	if !templates[0].Available || templates[0].Version != "17.6" || templates[0].Location == "" {
		t.Fatalf("template = %+v, want the installed engine described", templates[0])
	}

	findPostgres = func() (postgresBinaries, error) { return postgresBinaries{}, errors.New("not found") }
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
	createdMarker := filepath.Join(filepath.Dir(binaries.createdb), "database-created.txt")
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
	findPostgres = func() (postgresBinaries, error) { return postgresBinaries{}, errors.New("nothing here") }

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
	if out := supervisor.Output(instance.ID); out != nil {
		t.Fatalf("output = %v, want no live process", out)
	}

	binaries, err := findPostgres()
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	stopMarker := filepath.Join(filepath.Dir(binaries.pgctl), "stop-invoked.txt")
	if _, err := os.Stat(stopMarker); err != nil {
		t.Fatalf("graceful stop marker: %v", err)
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
	flag := filepath.Join(filepath.Dir(binaries.createdb), "first-attempt.txt")
	success := filepath.Join(filepath.Dir(binaries.createdb), "database-created.txt")
	if err := os.Remove(success); err != nil && !os.IsNotExist(err) {
		t.Fatalf("clear marker: %v", err)
	}
	if runtime.GOOS == "windows" {
		writeScript(t, binaries.createdb,
			"@echo off\r\n"+
				"if exist \""+flag+"\" (\r\n"+
				"  echo created>> \""+success+"\"\r\n"+
				"  exit /b 0\r\n"+
				")\r\n"+
				"type nul> \""+flag+"\"\r\n"+
				"echo the database system is starting up 1>&2\r\n"+
				"exit /b 1\r\n")
	} else {
		writeScript(t, binaries.createdb,
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
