package applications

import (
	"database/sql"
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"locafy.dev/desktop/internal/projects"
)

// Service exposes applications over the application binding layer. Validation
// and storage rules live in the package functions; process supervision lives in
// the supervisor.
type Service struct {
	database   *sql.DB
	supervisor *Supervisor
}

// NewService returns an applications service backed by database.
func NewService(database *sql.DB) *Service {
	service := &Service{database: database}
	service.supervisor = NewSupervisor(database, func(name string, data any) {
		if app := application.Get(); app != nil {
			app.Event.Emit(name, data)
		}
	})
	return service
}

// List returns a project's live applications with their dependencies.
func (s *Service) List(projectID string) ([]Application, error) {
	return List(s.database, projectID)
}

// Detect proposes the applications found in a project's folders. Nothing is
// saved or executed: the result is a suggestion the operator confirms.
func (s *Service) Detect(projectID string) ([]Detection, error) {
	project, err := projects.Get(s.database, projectID)
	if err != nil {
		return nil, err
	}
	return Detect(project.RootPath)
}

// Create saves a confirmed application.
func (s *Service) Create(projectID string, input CreateInput) (Application, error) {
	return Create(s.database, projectID, input)
}

// Remove stops an application if it is running, then takes it out of Locafy
// without touching its files.
func (s *Service) Remove(id string) error {
	if err := s.supervisor.Stop(id); err != nil && !errors.Is(err, ErrNotRunning) {
		return err
	}
	return Remove(s.database, id)
}

// SetDependencies replaces what an application depends on, refusing a cycle.
func (s *Service) SetDependencies(id string, dependsOn []string) error {
	return SetDependencies(s.database, id, dependsOn)
}

// SetRuntime pins or clears the runtime copy an application starts with.
func (s *Service) SetRuntime(id string, runtimeID string) error {
	return SetRuntime(s.database, id, runtimeID)
}

// Start runs an application's stored command in its stored folder.
func (s *Service) Start(id string) (Application, error) {
	return s.supervisor.Start(id)
}

// Stop kills an application's process tree.
func (s *Service) Stop(id string) error {
	return s.supervisor.Stop(id)
}

// Restart stops an application and starts it again from the same record.
func (s *Service) Restart(id string) (Application, error) {
	return s.supervisor.Restart(id)
}

// Output returns the recent output of a running application.
func (s *Service) Output(id string) []string {
	return s.supervisor.Output(id)
}

// StopAll kills every supervised process tree, so quitting Locafy leaves no
// server behind.
func (s *Service) StopAll() {
	s.supervisor.StopAll()
}

// StartProject starts a project's applications in dependency order.
func (s *Service) StartProject(projectID string) (*ProjectReport, error) {
	return s.supervisor.StartProject(projectID)
}

// StopProject stops a project's applications in reverse dependency order.
func (s *Service) StopProject(projectID string) (*ProjectReport, error) {
	return s.supervisor.StopProject(projectID)
}

// RestartProject stops a project and starts it again.
func (s *Service) RestartProject(projectID string) (*ProjectReport, error) {
	return s.supervisor.RestartProject(projectID)
}

// Lifecycle derives a project's state from its applications' statuses.
func (s *Service) Lifecycle(projectID string) (string, error) {
	return s.supervisor.ProjectState(projectID)
}

// Lifecycles derives a state word for every project that owns applications.
func (s *Service) Lifecycles() (map[string]string, error) {
	return s.supervisor.ProjectStates()
}

// ProjectUsage sums the CPU and memory the project's managed trees use now.
func (s *Service) ProjectUsage(projectID string) (*ProjectUsage, error) {
	return s.supervisor.ProjectUsage(projectID)
}
