package applications

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/projects"
	"locafy.dev/desktop/internal/services"
)

// sequence records the order starts and stops happened in, across the fake
// service runtime and the supervisor's own state events.
type sequence struct {
	mu    sync.Mutex
	steps []string
}

func (s *sequence) add(step string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = append(s.steps, step)
}

func (s *sequence) joined() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.steps, ",")
}

// fakeServices stands in for the service supervisor: it mutates the real
// records, so the orchestration's own reads and waiting behave as in production.
type fakeServices struct {
	database *sql.DB
	order    *sequence
	fail     map[string]error
	started  []string
	stopped  []string
	mu       sync.Mutex
}

func newFakeServices(database *sql.DB, order *sequence) *fakeServices {
	return &fakeServices{database: database, order: order, fail: map[string]error{}}
}

func (f *fakeServices) Start(id string) (services.Instance, error) {
	f.mu.Lock()
	f.started = append(f.started, id)
	f.mu.Unlock()
	f.order.add("service start")

	if err := f.fail[id]; err != nil {
		_, _ = f.database.Exec(`UPDATE services SET status = ?, error = ? WHERE id = ?`,
			services.StatusError, err.Error(), id)
		instance, _ := services.Get(f.database, id)
		return instance, err
	}
	_, _ = f.database.Exec(`UPDATE services
		SET status = ?, health = ?, health_detail = 'Answering.'
		WHERE id = ?`, services.StatusRunning, services.HealthHealthy, id)
	instance, err := services.Get(f.database, id)
	return instance, err
}

func (f *fakeServices) Stop(id string) error {
	f.mu.Lock()
	f.stopped = append(f.stopped, id)
	f.mu.Unlock()
	f.order.add("service stop")
	_, _ = f.database.Exec(`UPDATE services SET status = ?, health = '', health_detail = '' WHERE id = ?`,
		services.StatusStopped, id)
	return nil
}

// projectService records a PostgreSQL instance without starting one.
func projectService(t *testing.T, database *sql.DB, project projects.Project, name string, port int) services.Instance {
	t.Helper()

	id := db.NewID()
	_, err := database.Exec(`INSERT INTO services
			(id, project_id, name, engine, version, port, data_directory, database_name, user_name)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'locafy', 'locafy')`,
		id, project.ID, name, services.EnginePostgreSQL, "17.2", port,
		filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	instance, err := services.Get(database, id)
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	return instance
}

func TestProjectStartBringsServicesUpBeforeApplications(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	instance := projectService(t, database, project, "PostgreSQL", 5501)
	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	app := projectApplication(t, database, project, "api", live)

	order := &sequence{}
	fake := newFakeServices(database, order)
	supervisor := NewSupervisor(database, func(name string, data any) {
		if name != EventState {
			return
		}
		payload := data.(map[string]any)
		order.add("application " + payload["status"].(string))
	})
	supervisor.services = fake
	t.Cleanup(supervisor.StopAll)

	report, err := supervisor.StartProject(project.ID)
	if err != nil {
		t.Fatalf("start project: %v", err)
	}

	if got, want := strings.Join(report.Started, ","), "PostgreSQL,api"; got != want {
		t.Fatalf("started %q, want the service ahead of the application (%q)", got, want)
	}
	if len(fake.started) != 1 || fake.started[0] != instance.ID {
		t.Fatalf("service starts = %v, want the project's instance once", fake.started)
	}
	if steps := order.joined(); !strings.HasPrefix(steps, "service start,application running") {
		t.Fatalf("order = %q, want the service started before the application", steps)
	}
	if state, err := supervisor.ProjectState(project.ID); err != nil || state != ProjectRunning {
		t.Fatalf("state = %q (%v), want running once both are up", state, err)
	}
	if running, err := Get(database, app.ID); err != nil || running.Status != StatusRunning {
		t.Fatalf("application = %+v (%v), want it running", running, err)
	}

	entries, err := recentProjectActivity(database, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 || entries[0].Action != "Project started" ||
		!strings.Contains(entries[0].Detail, "started PostgreSQL, api") {
		t.Fatalf("activity = %+v, want the service reported with the application", entries)
	}
}

func TestProjectStartReportsAFailedServiceAndStillStartsApplications(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	instance := projectService(t, database, project, "PostgreSQL", 5502)
	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	projectApplication(t, database, project, "api", live)

	fake := newFakeServices(database, &sequence{})
	fake.fail[instance.ID] = errors.New("PostgreSQL is not installed on this machine")
	supervisor := NewSupervisor(database, nil)
	supervisor.services = fake
	t.Cleanup(supervisor.StopAll)

	report, err := supervisor.StartProject(project.ID)
	if err != nil {
		t.Fatalf("start project: %v", err)
	}
	if len(report.Failed) != 1 || !strings.HasPrefix(report.Failed[0], "PostgreSQL: ") {
		t.Fatalf("failed = %v, want the service named with its reason", report.Failed)
	}
	if got, want := strings.Join(report.Started, ","), "api"; got != want {
		t.Fatalf("started = %q, want the application still attempted (%q)", got, want)
	}
	if state, err := supervisor.ProjectState(project.ID); err != nil || state != ProjectPartial {
		t.Fatalf("state = %q (%v), want partial while a service is down", state, err)
	}
}

func TestProjectStopStopsApplicationsBeforeServices(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	instance := projectService(t, database, project, "PostgreSQL", 5503)
	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	projectApplication(t, database, project, "api", live)

	order := &sequence{}
	fake := newFakeServices(database, order)
	supervisor := NewSupervisor(database, func(name string, data any) {
		if name != EventState {
			return
		}
		payload := data.(map[string]any)
		order.add("application " + payload["status"].(string))
	})
	supervisor.services = fake
	if _, err := supervisor.StartProject(project.ID); err != nil {
		t.Fatalf("start project: %v", err)
	}
	t.Cleanup(supervisor.StopAll)

	report, err := supervisor.StopProject(project.ID)
	if err != nil {
		t.Fatalf("stop project: %v", err)
	}
	if got, want := strings.Join(report.Stopped, ","), "api,PostgreSQL"; got != want {
		t.Fatalf("stopped %q, want applications then services (%q)", got, want)
	}
	if len(fake.stopped) != 1 || fake.stopped[0] != instance.ID {
		t.Fatalf("service stops = %v, want the project's instance once", fake.stopped)
	}
	if steps := order.joined(); !strings.Contains(steps, "application stopping,application stopped,service stop") {
		t.Fatalf("order = %q, want the application stopped before the service", steps)
	}
	if state, err := supervisor.ProjectState(project.ID); err != nil || state != ProjectStopped {
		t.Fatalf("state = %q (%v), want stopped", state, err)
	}
}

func TestProjectStateCountsAServiceThatIsDown(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	instance := projectService(t, database, project, "PostgreSQL", 5504)
	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	app := projectApplication(t, database, project, "api", live)

	supervisor := NewSupervisor(database, nil)
	t.Cleanup(supervisor.StopAll)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, app.ID, StatusRunning)

	state, err := supervisor.ProjectState(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state != ProjectPartial {
		t.Fatalf("state = %q, want partial while the service is stopped", state)
	}

	if _, err := database.Exec(`UPDATE services SET status = ? WHERE id = ?`, services.StatusRunning, instance.ID); err != nil {
		t.Fatal(err)
	}
	state, err = supervisor.ProjectState(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state != ProjectRunning {
		t.Fatalf("state = %q, want running once the service is up too", state)
	}
}

func TestProjectStatesCoverAProjectWithOnlyServices(t *testing.T) {
	database := openDatabase(t)
	project := newProject(t, database)
	instance := projectService(t, database, project, "PostgreSQL", 5505)

	supervisor := NewSupervisor(database, nil)
	if _, err := database.Exec(`UPDATE services SET status = ? WHERE id = ?`, services.StatusRunning, instance.ID); err != nil {
		t.Fatal(err)
	}

	states, err := supervisor.ProjectStates()
	if err != nil {
		t.Fatal(err)
	}
	if states[project.ID] != ProjectRunning {
		t.Fatalf("state = %q, want running for a project with only services", states[project.ID])
	}
}
