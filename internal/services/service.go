package services

import (
	"database/sql"
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Service exposes service instances over the application binding layer. Storage
// rules live in the package functions; process supervision lives in the
// supervisor.
type Service struct {
	database   *sql.DB
	root       string
	supervisor *Supervisor
}

// NewService returns a services service backed by database.
func NewService(database *sql.DB) *Service {
	root, _ := DataRoot()
	service := &Service{database: database, root: root}
	service.supervisor = NewSupervisor(database, func(name string, data any) {
		if app := application.Get(); app != nil {
			app.Event.Emit(name, data)
		}
	})
	return service
}

// Catalogue lists the engines that can be attached, and whether each is
// installed on this machine.
func (s *Service) Catalogue() []Template {
	return Catalogue()
}

// List returns a project's services.
func (s *Service) List(projectID string) ([]Instance, error) {
	return List(s.database, projectID)
}

// Create attaches a service to a project without starting it.
func (s *Service) Create(projectID string, input CreateInput) (Instance, error) {
	root, err := s.rootDirectory()
	if err != nil {
		return Instance{}, err
	}
	return Create(s.database, root, projectID, input)
}

// Start initialises, if needed, and runs the instance's engine.
func (s *Service) Start(id string) (Instance, error) {
	return s.supervisor.Start(id)
}

// Stop shuts the instance down, preferring the engine's own stop command.
func (s *Service) Stop(id string) error {
	return s.supervisor.Stop(id)
}

// Restart stops and starts the same instance, keeping its data and port.
func (s *Service) Restart(id string) (Instance, error) {
	return s.supervisor.Restart(id)
}

// Remove stops the instance and deletes the record, keeping its data directory.
func (s *Service) Remove(id string) (Removal, error) {
	if err := s.supervisor.Stop(id); err != nil && !errors.Is(err, ErrNotRunning) {
		return Removal{}, err
	}
	return Remove(s.database, id)
}

// Output returns the recent output of a running instance.
func (s *Service) Output(id string) []string {
	return s.supervisor.Output(id)
}

// StopAll stops every supervised instance, so quitting Locafy leaves no server
// behind.
func (s *Service) StopAll() {
	s.supervisor.StopAll()
}

func (s *Service) rootDirectory() (string, error) {
	if s.root != "" {
		return s.root, nil
	}
	return DataRoot()
}
