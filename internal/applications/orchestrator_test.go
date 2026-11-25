package applications

import (
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/projects"
)

// runRecorder captures state events with the application that produced them,
// so an orchestration's order can be read back.
type runRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *runRecorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name != EventState {
		return
	}
	payload := data.(map[string]any)
	r.events = append(r.events, "state:"+payload["status"].(string)+":"+payload["applicationId"].(string))
}

func (r *runRecorder) joined() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.events, "\n")
}

// fastReady shortens the readiness ceilings, so a test does not wait seconds
// for a dependency that will never bind.
func fastReady(t *testing.T) {
	t.Helper()

	previousGrace, previousWait, previousPoll, previousSettle, previousMin := bindGrace, healthWait, readyPoll, settleWait, minSettle
	bindGrace, healthWait, readyPoll, settleWait, minSettle = 400*time.Millisecond, 400*time.Millisecond, 20*time.Millisecond, 500*time.Millisecond, 150*time.Millisecond
	t.Cleanup(func() {
		bindGrace, healthWait, readyPoll, settleWait, minSettle = previousGrace, previousWait, previousPoll, previousSettle, previousMin
	})
}

// projectApplication creates one runnable application in its own folder, named
// after the caller's choice.
func projectApplication(t *testing.T, database *sql.DB, project projects.Project, name string, command string) Application {
	t.Helper()

	makeFolder(t, project, name)
	app, err := Create(database, project.ID, CreateInput{
		Name:         name,
		Directory:    name,
		StartCommand: command,
	})
	if err != nil {
		t.Fatalf("create application %s: %v", name, err)
	}
	return app
}

func recentProjectActivity(database *sql.DB, projectID string) ([]activity.Entry, error) {
	return activity.RecentFor(database, activity.ScopeProject, projectID, 5)
}

func TestProjectStartFollowsTheDependencyOrder(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	store := projectApplication(t, database, project, "database", live)
	api := projectApplication(t, database, project, "api", live)
	web := projectApplication(t, database, project, "web", live)
	if err := SetDependencies(database, api.ID, []string{store.ID}); err != nil {
		t.Fatal(err)
	}
	if err := SetDependencies(database, web.ID, []string{store.ID, api.ID}); err != nil {
		t.Fatal(err)
	}

	events := &runRecorder{}
	supervisor := NewSupervisor(database, events.emit)
	report, err := supervisor.StartProject(project.ID)
	if err != nil {
		t.Fatalf("start project: %v", err)
	}
	t.Cleanup(func() { _, _ = supervisor.StopProject(project.ID) })

	if got, want := strings.Join(report.Started, ","), "database,api,web"; got != want {
		t.Fatalf("started %q, want %q", got, want)
	}
	if len(report.Failed) != 0 || len(report.Skipped) != 0 {
		t.Fatalf("report = %+v, want every application started", report)
	}
	var runningOrder []string
	for _, event := range strings.Split(events.joined(), "\n") {
		if strings.HasPrefix(event, "state:running:") {
			runningOrder = append(runningOrder, strings.TrimPrefix(event, "state:running:"))
		}
	}
	if got, want := strings.Join(runningOrder, ","), store.ID+","+api.ID+","+web.ID; got != want {
		t.Fatalf("running order %q, want %q", got, want)
	}
}

// A runtime removed after it was assigned fails its own application before any
// other work runs, and the reason is written on the record so it survives a
// refresh. The rest of the project still starts.
func TestProjectStartChecksRuntimesBeforeStarting(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	broken := projectApplication(t, database, project, "api", live)
	projectApplication(t, database, project, "web", live)
	// Written straight to the column: the assignment was valid when it was
	// made, and the copy is gone now, which SetRuntime would refuse.
	if _, err := database.Exec(`UPDATE applications SET runtime_id = ? WHERE id = ?`,
		`node@C:\missing\node.exe`, broken.ID); err != nil {
		t.Fatalf("assign runtime: %v", err)
	}

	supervisor := NewSupervisor(database, nil)
	t.Cleanup(func() { _, _ = supervisor.StopProject(project.ID) })
	report, err := supervisor.StartProject(project.ID)
	if err != nil {
		t.Fatalf("start project: %v", err)
	}

	if len(report.Failed) != 1 || !strings.Contains(report.Failed[0], "api") ||
		!strings.Contains(report.Failed[0], "assigned runtime is not available") {
		t.Fatalf("failed = %q, want the broken runtime reported for api", report.Failed)
	}
	if got := strings.Join(report.Started, ","); got != "web" {
		t.Fatalf("started = %q, want the rest of the project to run", got)
	}

	record, err := Get(database, broken.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if record.Status != StatusError || !strings.Contains(record.Error, "assigned runtime is not available") {
		t.Fatalf("record = %q/%q, want the runtime failure written on it", record.Status, record.Error)
	}
}

func TestProjectStartSkipsDependentsOfAFailedApplication(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	// The crash must still be classified from the record when the runner is
	// loaded, so this test observes a start as long as production does.
	minSettle = 2 * time.Second
	settleWait = 3 * time.Second
	project := newProject(t, database)

	broken := quoteCommand(script(t, t.TempDir(), "broken", 3, false))
	api := projectApplication(t, database, project, "api", broken)
	worker := projectApplication(t, database, project, "worker", quoteCommand(script(t, t.TempDir(), "worker", 0, true)))
	if err := SetDependencies(database, worker.ID, []string{api.ID}); err != nil {
		t.Fatal(err)
	}

	supervisor := NewSupervisor(database, nil)
	report, err := supervisor.StartProject(project.ID)
	if err != nil {
		t.Fatalf("start project: %v", err)
	}
	t.Cleanup(supervisor.StopAll)

	if len(report.Failed) != 1 || !strings.HasPrefix(report.Failed[0], "api:") {
		t.Fatalf("failed = %v, want api with its reason", report.Failed)
	}
	if len(report.Skipped) != 1 || !strings.HasPrefix(report.Skipped[0], "worker:") {
		t.Fatalf("skipped = %v, want worker skipped", report.Skipped)
	}
	workerRecord, err := Get(database, worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	if workerRecord.Status != StatusStopped || workerRecord.Error == "" {
		t.Fatalf("worker = %q (%q), want stopped with the skip reason kept", workerRecord.Status, workerRecord.Error)
	}
}

func TestProjectStartWaitsForADependencyToAnswer(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	store := projectApplication(t, database, project, "database", live)
	api := projectApplication(t, database, project, "api", live)
	if err := SetDependencies(database, api.ID, []string{store.ID}); err != nil {
		t.Fatal(err)
	}

	// The dependency binds but never answers: its dependent must be skipped
	// rather than started blind.
	fakePorts(t, []listener{{Port: 4700, PID: 4242, Process: "node"}},
		func(pid int, root int) bool { return pid == 4242 && root > 0 })
	fakeHealth(t, func(port int) error { return errors.New("connection refused") })

	supervisor := NewSupervisor(database, nil)
	report, err := supervisor.StartProject(project.ID)
	if err != nil {
		t.Fatalf("start project: %v", err)
	}
	t.Cleanup(supervisor.StopAll)

	if len(report.Skipped) != 1 || !strings.Contains(report.Skipped[0], "does not answer on port 4700") {
		t.Fatalf("skipped = %v, want api skipped for a silent dependency", report.Skipped)
	}
	apiRecord, err := Get(database, api.ID)
	if err != nil {
		t.Fatal(err)
	}
	if apiRecord.Status != StatusStopped {
		t.Fatalf("api = %q, want it left stopped", apiRecord.Status)
	}
}

func TestProjectStartAcceptsAPortlessDependency(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	worker := projectApplication(t, database, project, "worker", live)
	api := projectApplication(t, database, project, "api", live)
	if err := SetDependencies(database, api.ID, []string{worker.ID}); err != nil {
		t.Fatal(err)
	}

	// Nothing listens anywhere: the worker cannot prove more than running.
	fakePorts(t, nil, func(pid int, root int) bool { return true })

	supervisor := NewSupervisor(database, nil)
	report, err := supervisor.StartProject(project.ID)
	if err != nil {
		t.Fatalf("start project: %v", err)
	}
	t.Cleanup(supervisor.StopAll)

	if len(report.Started) != 2 {
		t.Fatalf("report = %+v, want both applications started", report)
	}
}

func TestProjectStartSkipsApplicationsAlreadyRunning(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	app := projectApplication(t, database, project, "api", live)

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, app.ID, StatusRunning)
	t.Cleanup(supervisor.StopAll)

	report, err := supervisor.StartProject(project.ID)
	if err != nil {
		t.Fatalf("start project: %v", err)
	}
	if got, want := strings.Join(report.AlreadyRunning, ","), app.Name; got != want {
		t.Fatalf("already running = %q, want %q", got, want)
	}
	if len(report.Started) != 0 {
		t.Fatalf("started = %v, want nothing started twice", report.Started)
	}
}

func TestProjectStopRunsInTheReverseOrder(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	store := projectApplication(t, database, project, "database", live)
	api := projectApplication(t, database, project, "api", live)
	web := projectApplication(t, database, project, "web", live)
	if err := SetDependencies(database, api.ID, []string{store.ID}); err != nil {
		t.Fatal(err)
	}
	if err := SetDependencies(database, web.ID, []string{store.ID, api.ID}); err != nil {
		t.Fatal(err)
	}

	events := &runRecorder{}
	supervisor := NewSupervisor(database, events.emit)
	if _, err := supervisor.StartProject(project.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{store.ID, api.ID, web.ID} {
		waitForStatus(t, database, id, StatusRunning)
	}

	report, err := supervisor.StopProject(project.ID)
	if err != nil {
		t.Fatalf("stop project: %v", err)
	}

	if got, want := strings.Join(report.Stopped, ","), "web,api,database"; got != want {
		t.Fatalf("stopped %q, want %q", got, want)
	}
	var stoppingOrder []string
	for _, event := range strings.Split(events.joined(), "\n") {
		if strings.HasPrefix(event, "state:stopping:") {
			stoppingOrder = append(stoppingOrder, strings.TrimPrefix(event, "state:stopping:"))
		}
	}
	if got, want := strings.Join(stoppingOrder, ","), web.ID+","+api.ID+","+store.ID; got != want {
		t.Fatalf("stopping order %q, want %q", got, want)
	}
}

func TestProjectRestartStopsAndStartsTheApplications(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	app := projectApplication(t, database, project, "api", live)

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.StartProject(project.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, app.ID, StatusRunning)

	report, err := supervisor.RestartProject(project.ID)
	if err != nil {
		t.Fatalf("restart project: %v", err)
	}
	t.Cleanup(supervisor.StopAll)

	if got, want := strings.Join(report.Stopped, ","), app.Name; got != want {
		t.Fatalf("stopped = %q, want %q", got, want)
	}
	if got, want := strings.Join(report.Started, ","), app.Name; got != want {
		t.Fatalf("started = %q, want %q", got, want)
	}
	if after := waitForStatus(t, database, app.ID, StatusRunning); after.StartedAt == "" {
		t.Fatal("the restarted run kept no start time")
	}
}

func TestProjectActionsRefuseWhileOneIsRunning(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	projectApplication(t, database, project, "api", live)

	supervisor := NewSupervisor(database, nil)
	release, claimErr := supervisor.claimProject(project.ID, "starting")
	if claimErr != nil {
		t.Fatal(claimErr)
	}

	if _, err := supervisor.StartProject(project.ID); !errors.Is(err, ErrProjectBusy) {
		t.Fatalf("start while busy: %v, want ErrProjectBusy", err)
	}
	if _, err := supervisor.StopProject(project.ID); !errors.Is(err, ErrProjectBusy) {
		t.Fatalf("stop while busy: %v, want ErrProjectBusy", err)
	}

	release()
	if _, err := supervisor.StartProject(project.ID); err != nil {
		t.Fatalf("start after release: %v", err)
	}
	t.Cleanup(supervisor.StopAll)
}

func TestProjectStateWords(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	broken := quoteCommand(script(t, t.TempDir(), "broken", 3, false))
	one := projectApplication(t, database, project, "api", live)
	two := projectApplication(t, database, project, "worker", broken)

	supervisor := NewSupervisor(database, nil)
	state := func(want string) {
		t.Helper()
		got, err := supervisor.ProjectState(project.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("state = %q, want %q", got, want)
		}
	}

	state(ProjectStopped)
	if _, err := supervisor.Start(one.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, one.ID, StatusRunning)
	state(ProjectPartial)
	if _, err := supervisor.Start(two.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, two.ID, StatusError)
	state(ProjectPartial)
	if err := supervisor.Stop(one.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, one.ID, StatusStopped)
	state(ProjectError)
	t.Cleanup(supervisor.StopAll)
}

func TestProjectStatesCoverEveryProject(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	first := newProject(t, database)
	second := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	app := projectApplication(t, database, first, "api", live)
	broken := projectApplication(t, database, second, "worker", quoteCommand(script(t, t.TempDir(), "broken", 3, false)))

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, app.ID, StatusRunning)
	if _, err := supervisor.Start(broken.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, broken.ID, StatusError)
	t.Cleanup(func() { supervisor.StopAll() })

	states, err := supervisor.ProjectStates()
	if err != nil {
		t.Fatal(err)
	}
	if states[first.ID] != ProjectRunning {
		t.Fatalf("first = %q, want running", states[first.ID])
	}
	if states[second.ID] != ProjectError {
		t.Fatalf("second = %q, want error", states[second.ID])
	}
}

func TestProjectStartRecordsActivity(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	projectApplication(t, database, project, "api", live)

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.StartProject(project.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(supervisor.StopAll)

	entries, err := recentProjectActivity(database, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 || entries[0].Action != "Project started" {
		t.Fatalf("entries = %+v, want Project started first", entries)
	}
}
