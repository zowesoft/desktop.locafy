package applications

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/projects"
	"locafy.dev/desktop/internal/services"
)

// Derived project states, computed from the component statuses rather than
// stored: the CHECK vocabulary on projects.lifecycle_state cannot express
// "partial", and a stored single state would drift from what it summarises.
const (
	ProjectRunning  = "running"
	ProjectPartial  = "partial"
	ProjectStopped  = "stopped"
	ProjectError    = "error"
	ProjectStarting = "starting"
	ProjectStopping = "stopping"
)

var (
	// ErrProjectBusy is returned when a project orchestration is asked to run
	// while another one for the same project is still in progress.
	ErrProjectBusy = errors.New("a project start or stop is already in progress")

	// bindGrace is how long a running dependency may take to bind its first
	// port before it is treated as portless.
	bindGrace = 10 * time.Second
	// healthWait is how long a bound dependency may take to answer before its
	// dependents are skipped.
	healthWait = 20 * time.Second
	// settleWait is how long a just-started application is watched for an
	// immediate exit before its dependents are considered.
	settleWait = 2 * time.Second
	// minSettle is how long a start is observed even when it already reports
	// running, so a crash on boot is seen before dependents start.
	minSettle = 250 * time.Millisecond
	// readyPoll is how often a waiting orchestrator re-reads the record.
	readyPoll = 200 * time.Millisecond
)

// ProjectReport summarises one orchestration run, application by application.
type ProjectReport struct {
	ProjectID      string   `json:"projectID"`
	Started        []string `json:"started"`
	AlreadyRunning []string `json:"alreadyRunning"`
	Skipped        []string `json:"skipped"`
	Failed         []string `json:"failed"`
	Stopped        []string `json:"stopped"`
	// KeptRunning and LeftStopped exist for a restart, which leaves the
	// project's services exactly as they are. Naming them keeps the activity
	// entry from being silent about the database it did not touch.
	KeptRunning []string `json:"keptRunning"`
	LeftStopped []string `json:"leftStopped"`
}

// startProject brings the project's services up first, then its runnable
// applications in dependency order. It is the body behind StartProject.
func (s *Supervisor) startProject(projectID string) (*ProjectReport, error) {
	if _, err := projects.Get(s.database, projectID); err != nil {
		return nil, err
	}

	report := &ProjectReport{ProjectID: projectID}
	// Services are the project's foundations: every application sees a running
	// database before its own command is executed.
	s.startServices(projectID, report)
	if err := s.startApplications(projectID, report); err != nil {
		return nil, err
	}
	return report, nil
}

// startApplications runs the project's applications in dependency order and
// reports each one into report. It is separate from startProject because a
// restart runs it without touching services.
func (s *Supervisor) startApplications(projectID string, report *ProjectReport) error {
	apps, err := List(s.database, projectID)
	if err != nil {
		return err
	}
	for _, app := range startOrder(apps) {
		current, err := Get(s.database, app.ID)
		if err != nil {
			return err
		}
		switch current.Status {
		case StatusRunning:
			report.AlreadyRunning = append(report.AlreadyRunning, current.Name)
			continue
		case StatusStarting, StatusStopping:
			report.Skipped = append(report.Skipped,
				fmt.Sprintf("%s: already %s", current.Name, current.Status))
			continue
		}

		reason, ready := s.dependenciesReady(current)
		if !ready {
			s.note(current.ID, reason)
			report.Skipped = append(report.Skipped, fmt.Sprintf("%s: %s", current.Name, reason))
			continue
		}

		started, err := s.Start(current.ID)
		if err != nil {
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %s", current.Name, failureReason(started)))
			continue
		}
		if reason, crashed := s.settle(current.ID); crashed {
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %s", current.Name, reason))
			continue
		}
		report.Started = append(report.Started, started.Name)
	}
	return nil
}

// stopProject stops the project's running applications in reverse dependency
// order, then its services.
func (s *Supervisor) stopProject(projectID string) (*ProjectReport, error) {
	if _, err := projects.Get(s.database, projectID); err != nil {
		return nil, err
	}

	report := &ProjectReport{ProjectID: projectID}
	if err := s.stopApplications(projectID, report); err != nil {
		return nil, err
	}
	s.stopServices(projectID, report)
	return report, nil
}

// stopApplications stops the project's running applications in reverse
// dependency order and reports each one into report.
func (s *Supervisor) stopApplications(projectID string, report *ProjectReport) error {
	apps, err := List(s.database, projectID)
	if err != nil {
		return err
	}
	for _, app := range reverse(startOrder(apps)) {
		current, err := Get(s.database, app.ID)
		if err != nil {
			return err
		}
		if current.Status != StatusRunning && current.Status != StatusStopping {
			continue
		}
		if err := s.Stop(current.ID); err != nil {
			if errors.Is(err, ErrNotRunning) {
				continue
			}
			report.Failed = append(report.Failed, fmt.Sprintf("%s: %s", current.Name, err))
			continue
		}
		report.Stopped = append(report.Stopped, current.Name)
	}
	return nil
}

// StartProject starts the project's applications in dependency order and
// records the run at project scope. Only one orchestration runs per project.
func (s *Supervisor) StartProject(projectID string) (*ProjectReport, error) {
	release, err := s.claimProject(projectID, "starting")
	if err != nil {
		return nil, err
	}
	defer release()

	report, err := s.startProject(projectID)
	s.recordRun(projectID, "Project started", report, err)
	return report, err
}

// StopProject stops the project's applications in reverse dependency order.
func (s *Supervisor) StopProject(projectID string) (*ProjectReport, error) {
	release, err := s.claimProject(projectID, "stopping")
	if err != nil {
		return nil, err
	}
	defer release()

	report, err := s.stopProject(projectID)
	s.recordRun(projectID, "Project stopped", report, err)
	return report, err
}

// RestartProject cycles the project's applications and leaves its services
// alone: a restart is a reload of the code the operator is working on, not a
// reason to bounce a database that is holding their data.
func (s *Supervisor) RestartProject(projectID string) (*ProjectReport, error) {
	release, err := s.claimProject(projectID, "restarting")
	if err != nil {
		return nil, err
	}
	defer release()

	report, err := s.restartProject(projectID)
	s.recordRun(projectID, "Project restarted", report, err)
	return report, err
}

// restartProject stops and starts the applications, and records what it did to
// the services without doing anything to them: a running service keeps running,
// and one the operator stopped stays stopped (Start is the button that brings
// services up).
func (s *Supervisor) restartProject(projectID string) (*ProjectReport, error) {
	if _, err := projects.Get(s.database, projectID); err != nil {
		return nil, err
	}

	report := &ProjectReport{ProjectID: projectID}
	s.noteServiceDisposition(projectID, report)
	if err := s.stopApplications(projectID, report); err != nil {
		return report, err
	}
	if err := s.startApplications(projectID, report); err != nil {
		return report, err
	}
	return report, nil
}

// claimProject marks a project mid-orchestration, or refuses when one is
// already running. The returned release clears the mark.
func (s *Supervisor) claimProject(projectID string, phase string) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy == nil {
		s.busy = map[string]string{}
	}
	if taken, busy := s.busy[projectID]; busy {
		return nil, fmt.Errorf("%w: the project is %s", ErrProjectBusy, taken)
	}
	s.busy[projectID] = phase
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.busy, projectID)
	}, nil
}

// recordRun writes the activity entry for an orchestration run: the summary
// reaches the bell, the per-application detail lives on the records.
func (s *Supervisor) recordRun(projectID string, action string, report *ProjectReport, err error) {
	detail := ""
	if err != nil {
		detail = err.Error()
	} else if report != nil {
		detail = strings.Join(report.summary(), "; ")
	}
	if detail == "" {
		detail = "nothing to run"
	}
	_ = activity.Log(s.database, activity.ScopeProject, projectID, action, detail)
}

// summary words the report for an activity entry. A stop is worded before a
// start so a restart reads in the order it happened: stopped, started, and what
// it left alone.
func (r *ProjectReport) summary() []string {
	words := make([]string, 0, 7)
	if len(r.Stopped) > 0 {
		words = append(words, fmt.Sprintf("stopped %s", joinNames(r.Stopped)))
	}
	if len(r.Started) > 0 {
		words = append(words, fmt.Sprintf("started %s", joinNames(r.Started)))
	}
	if len(r.AlreadyRunning) > 0 {
		words = append(words, fmt.Sprintf("already running: %s", joinNames(r.AlreadyRunning)))
	}
	if len(r.KeptRunning) > 0 {
		words = append(words, fmt.Sprintf("kept %s running", joinNames(r.KeptRunning)))
	}
	if len(r.LeftStopped) > 0 {
		words = append(words, fmt.Sprintf("left %s stopped", joinNames(r.LeftStopped)))
	}
	if len(r.Skipped) > 0 {
		words = append(words, fmt.Sprintf("skipped %s", joinNames(r.Skipped)))
	}
	if len(r.Failed) > 0 {
		words = append(words, fmt.Sprintf("failed %s", joinNames(r.Failed)))
	}
	return words
}

func joinNames(entries []string) string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name, _, _ := strings.Cut(entry, ":")
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// startOrder returns the project's runnable applications with every
// dependency ahead of its dependents. Name order breaks ties so the order is
// stable. Cycle-free order is guaranteed by the validated graph; leftover
// nodes would only surface if that guarantee was broken, and they are appended
// rather than dropped so nothing silently disappears from a run.
func startOrder(apps []Application) []Application {
	runnable := make([]Application, 0, len(apps))
	byID := make(map[string]Application, len(apps))
	for _, app := range apps {
		if strings.TrimSpace(app.StartCommand) == "" {
			continue
		}
		runnable = append(runnable, app)
		byID[app.ID] = app
	}

	dependents := make(map[string][]string, len(runnable))
	pending := make(map[string]int, len(runnable))
	for _, app := range runnable {
		for _, dependency := range app.DependsOn {
			if _, inRun := byID[dependency]; !inRun {
				continue
			}
			dependents[dependency] = append(dependents[dependency], app.ID)
			pending[app.ID]++
		}
	}

	var ready []string
	for _, app := range runnable {
		if pending[app.ID] == 0 {
			ready = append(ready, app.ID)
		}
	}
	sortIDsByApp(ready, byID)

	ordered := make([]Application, 0, len(runnable))
	seen := make(map[string]bool, len(runnable))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		ordered = append(ordered, byID[id])
		next := append([]string(nil), dependents[id]...)
		sortIDsByApp(next, byID)
		ready = append(ready, next...)
	}
	if len(ordered) < len(runnable) {
		for _, app := range runnable {
			if !seen[app.ID] {
				ordered = append(ordered, app)
			}
		}
	}
	return ordered
}

func sortIDsByApp(ids []string, byID map[string]Application) {
	sort.Slice(ids, func(i, j int) bool {
		return strings.ToLower(byID[ids[i]].Name) < strings.ToLower(byID[ids[j]].Name)
	})
}

func reverse(apps []Application) []Application {
	reversed := make([]Application, len(apps))
	for i, app := range apps {
		reversed[len(apps)-1-i] = app
	}
	return reversed
}

// dependenciesReady reports whether every dependency the application needs is
// running and, where it listens, answering. The returned reason names the
// dependency that is not ready.
func (s *Supervisor) dependenciesReady(app Application) (string, bool) {
	for _, id := range app.DependsOn {
		reason, ready := s.dependencyReady(id)
		if !ready {
			return reason, false
		}
	}
	return "", true
}

func (s *Supervisor) dependencyReady(id string) (string, bool) {
	runningDeadline := time.Now().Add(bindGrace + healthWait)
	portDeadline := time.Now().Add(bindGrace)
	var healthDeadline time.Time
	for {
		dependency, err := Get(s.database, id)
		if err != nil {
			return "its dependency could not be read", false
		}
		name := dependency.Name
		switch dependency.Status {
		case StatusError, StatusStopped:
			if dependency.Error != "" {
				return fmt.Sprintf("%s is not running: %s", name, dependency.Error), false
			}
			return fmt.Sprintf("%s is not running", name), false
		case StatusStopping:
			return fmt.Sprintf("%s is stopping", name), false
		case StatusStarting:
			if time.Now().After(runningDeadline) {
				return fmt.Sprintf("%s did not reach running in time", name), false
			}
		case StatusRunning:
			if dependency.Port != nil {
				if dependency.Health == HealthHealthy {
					return "", true
				}
				if healthDeadline.IsZero() {
					healthDeadline = time.Now().Add(healthWait)
				}
				if time.Now().After(healthDeadline) {
					return fmt.Sprintf("%s does not answer on port %d", name, *dependency.Port), false
				}
			} else if healthDeadline.IsZero() {
				if time.Now().After(portDeadline) {
					// The grace passed with no port: a worker is as ready as
					// it can ever be.
					return "", true
				}
			} else if time.Now().After(healthDeadline) {
				// A port that appeared and went away again leaves running as
				// the only provable state.
				return "", true
			}
		}
		time.Sleep(readyPoll)
	}
}

// settle watches a just-started application for an immediate exit, so a
// command that crashes on boot fails its dependents instead of appearing as a
// running dependency. It returns at the first reading of running, stopped or
// error: the happy path costs one poll.
func (s *Supervisor) settle(id string) (string, bool) {
	deadline := time.Now().Add(settleWait)
	observed := time.Now().Add(minSettle)
	for {
		app, err := Get(s.database, id)
		if err != nil {
			return "", false
		}
		switch app.Status {
		case StatusError:
			return failureReason(app), true
		case StatusStopped:
			return "exited during startup", true
		case StatusRunning:
			if time.Now().After(observed) {
				return "", false
			}
		}
		if time.Now().After(deadline) {
			return "", false
		}
		time.Sleep(readyPoll)
	}
}

// note leaves the reason an application was skipped on its record, so it is
// still visible after a refresh, and announces the change.
func (s *Supervisor) note(id string, message string) {
	s.update(id, StatusStopped, nil, message, false)
	s.emitState(id, StatusStopped, nil, message)
}

func failureReason(app Application) string {
	if app.Error != "" {
		return app.Error
	}
	return "the application did not start"
}

// ProjectState derives the project's state from its services and applications:
// one word per component set, with "partial" where the honest answer is some up,
// some not.
func (s *Supervisor) ProjectState(projectID string) (string, error) {
	if _, err := projects.Get(s.database, projectID); err != nil {
		return "", err
	}
	apps, err := List(s.database, projectID)
	if err != nil {
		return "", err
	}
	instances, err := services.List(s.database, projectID)
	if err != nil {
		return "", err
	}
	return deriveProjectState(apps, instances), nil
}

// ProjectStates derives a word for every project that owns applications, for
// the dashboard's lifecycle badges.
func (s *Supervisor) ProjectStates() (map[string]string, error) {
	rows, err := s.database.Query(`SELECT project_id, status, start_command FROM applications WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("list application states: %w", err)
	}
	defer rows.Close()

	grouped := map[string][]Application{}
	for rows.Next() {
		var projectID, status, command string
		if err := rows.Scan(&projectID, &status, &command); err != nil {
			return nil, fmt.Errorf("scan application state: %w", err)
		}
		grouped[projectID] = append(grouped[projectID], Application{
			Status: status, StartCommand: command,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list application states: %w", err)
	}

	instances, err := s.serviceStates()
	if err != nil {
		return nil, err
	}

	states := make(map[string]string, len(grouped))
	for projectID, apps := range grouped {
		states[projectID] = deriveProjectState(apps, instances[projectID])
	}
	// A project can own services without applications; it still needs a word.
	for projectID, list := range instances {
		if _, seen := states[projectID]; !seen {
			states[projectID] = deriveProjectState(nil, list)
		}
	}
	return states, nil
}

// serviceStates groups every live service by project, reading only what the
// derived state needs.
func (s *Supervisor) serviceStates() (map[string][]services.Instance, error) {
	rows, err := s.database.Query(`SELECT project_id, status FROM services WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("list service states: %w", err)
	}
	defer rows.Close()

	grouped := map[string][]services.Instance{}
	for rows.Next() {
		var projectID, status string
		if err := rows.Scan(&projectID, &status); err != nil {
			return nil, fmt.Errorf("scan service state: %w", err)
		}
		grouped[projectID] = append(grouped[projectID], services.Instance{Status: status})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list service states: %w", err)
	}
	return grouped, nil
}

func deriveProjectState(apps []Application, instances []services.Instance) string {
	runnable := 0
	starting, stopping, running, errored := 0, 0, 0, 0
	for _, app := range apps {
		if strings.TrimSpace(app.StartCommand) == "" {
			continue
		}
		runnable++
		switch app.Status {
		case StatusStarting:
			starting++
		case StatusStopping:
			stopping++
		case StatusRunning:
			running++
		case StatusError:
			errored++
		}
	}
	for _, instance := range instances {
		runnable++
		switch instance.Status {
		case services.StatusStarting:
			starting++
		case services.StatusStopping:
			stopping++
		case services.StatusRunning:
			running++
		case services.StatusError:
			errored++
		}
	}
	switch {
	case runnable == 0:
		return ProjectStopped
	case starting > 0:
		return ProjectStarting
	case stopping > 0:
		return ProjectStopping
	case running == 0 && errored > 0:
		return ProjectError
	case running == runnable:
		return ProjectRunning
	case running > 0:
		return ProjectPartial
	default:
		return ProjectStopped
	}
}
