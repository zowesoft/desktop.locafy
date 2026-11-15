package projects

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/settings"
)

// WorkspaceRootKey stores the operator's chosen default location for
// managed projects.
const WorkspaceRootKey = "projects.workspace_root"

// Service exposes projects over the application binding layer. Validation and
// storage rules live in the package functions.
type Service struct {
	database *sql.DB
}

// NewService returns a projects service backed by database.
func NewService(database *sql.DB) *Service {
	return &Service{database: database}
}

// List returns live projects ordered by name.
func (s *Service) List() ([]Project, error) {
	return List(s.database)
}

// Create makes a managed project inside the workspace root: the stored
// preference when the operator has chosen one, the platform default
// otherwise.
func (s *Service) Create(name string) (Project, error) {
	dir, err := s.workspaceDir()
	if err != nil {
		return Project{}, fmt.Errorf("resolve projects directory: %w", err)
	}
	return Create(s.database, dir, name)
}

// WorkspaceDir returns where new managed projects are created.
func (s *Service) WorkspaceDir() (string, error) {
	return s.workspaceDir()
}

// SetWorkspaceDir stores the operator's default location for new managed
// projects. An empty path restores the platform default. Existing projects
// keep their recorded root.
func (s *Service) SetWorkspaceDir(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return settings.SetString(s.database, WorkspaceRootKey, "")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: %q", ErrWorkspaceInvalid, path)
	}
	return settings.SetString(s.database, WorkspaceRootKey, path)
}

// workspaceDir returns the stored workspace root when one is set, and the
// platform default otherwise.
func (s *Service) workspaceDir() (string, error) {
	dir, err := settings.String(s.database, WorkspaceRootKey)
	if errors.Is(err, settings.ErrNotFound) {
		return db.DefaultProjectsDir()
	}
	if err != nil {
		return "", err
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return db.DefaultProjectsDir()
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("%w: %q", ErrWorkspaceInvalid, dir)
	}
	return dir, nil
}

// Link registers an existing directory anywhere on the machine.
func (s *Service) Link(rootPath string) (Project, error) {
	return Link(s.database, strings.TrimSpace(rootPath))
}

// Rename changes a project's display name.
func (s *Service) Rename(id string, name string) (Project, error) {
	return Rename(s.database, id, name)
}

// ChangeRoot points a linked project at another directory.
func (s *Service) ChangeRoot(id string, rootPath string) (Project, error) {
	return ChangeRoot(s.database, id, rootPath)
}

// Remove takes a project out of Locafy without touching its files.
func (s *Service) Remove(id string) error {
	return Remove(s.database, id)
}
