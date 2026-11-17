package applications

import (
	"database/sql"

	"locafy.dev/desktop/internal/projects"
)

// Service exposes applications over the application binding layer. Validation
// and storage rules live in the package functions.
type Service struct {
	database *sql.DB
}

// NewService returns an applications service backed by database.
func NewService(database *sql.DB) *Service {
	return &Service{database: database}
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

// Remove takes an application out of Locafy without touching its files.
func (s *Service) Remove(id string) error {
	return Remove(s.database, id)
}

// SetDependencies replaces what an application depends on, refusing a cycle.
func (s *Service) SetDependencies(id string, dependsOn []string) error {
	return SetDependencies(s.database, id, dependsOn)
}
