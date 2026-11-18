// Package projects owns project records: the primary product object that
// applications, services and domains belong to. A project is either managed
// (Locafy created its directory inside the workspace) or linked (an existing
// directory elsewhere), per ADR-0007.
package projects

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/settings"
)

// Kind records who owns a project's directory.
const (
	KindManaged = "managed"
	KindLinked  = "linked"
)

// ManifestFile is the portable project definition written into managed
// projects only.
const ManifestFile = "locafy.project.json"

var (
	ErrNameRequired     = errors.New("a project needs a name")
	ErrPathRequired     = errors.New("a project needs a root directory")
	ErrPathMissing      = errors.New("the selected directory does not exist")
	ErrPathNotDir       = errors.New("the selected path is not a directory")
	ErrPathInUse        = errors.New("another project already uses this directory")
	ErrNotFound         = errors.New("project not found")
	ErrManagedRootFixed = errors.New("a managed project keeps the directory Locafy created; link the project to use another directory")
	ErrWorkspaceInvalid = errors.New("the workspace location must be an absolute path")
)

// Project is one project record.
type Project struct {
	ID             string
	Name           string
	RootPath       string
	Description    string
	Kind           string
	Status         string
	LifecycleState string
	CreatedAt      string
	UpdatedAt      string
}

// List returns live projects, ordered by name.
func List(database *sql.DB) ([]Project, error) {
	rows, err := database.Query(`SELECT id, name, root_path, description, kind, status, lifecycle_state, created_at, updated_at
		FROM projects WHERE deleted_at IS NULL ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		project, err := scan(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

// Get returns one live project, or ErrNotFound.
func Get(database *sql.DB, id string) (Project, error) {
	row := database.QueryRow(`SELECT id, name, root_path, description, kind, status, lifecycle_state, created_at, updated_at
		FROM projects WHERE id = ? AND deleted_at IS NULL`, id)
	project, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return project, err
}

// Create makes a managed project: a directory under projectsDir named from
// the project name, plus its manifest and database record.
func Create(database *sql.DB, projectsDir string, name string) (Project, error) {
	name, err := cleanName(name)
	if err != nil {
		return Project{}, err
	}
	if strings.TrimSpace(projectsDir) == "" {
		return Project{}, ErrPathRequired
	}

	root, err := allocateDir(projectsDir, name)
	if err != nil {
		return Project{}, err
	}

	project, err := RegisterManagedAt(database, name, root)
	if err != nil {
		// The directory exists only because this call created it.
		_ = os.RemoveAll(root)
		return Project{}, err
	}
	return project, nil
}

// RegisterManagedAt records a managed project whose directory a framework
// installer has already created, so the project exists only once its files do.
func RegisterManagedAt(database *sql.DB, name string, root string) (Project, error) {
	name, err := cleanName(name)
	if err != nil {
		return Project{}, err
	}
	if strings.TrimSpace(root) == "" {
		return Project{}, ErrPathRequired
	}

	project, err := insert(database, name, root, KindManaged)
	if err != nil {
		return Project{}, err
	}
	if err := writeManifest(root, project); err != nil {
		return project, err
	}
	if err := activity.Log(database, activity.ScopeProject, project.ID, "Project created", project.Name); err != nil {
		return project, err
	}
	return project, nil
}

// AllocateManagedPath returns a unique path for a new managed project without
// creating it, so an installer can create the folder itself and a failed
// scaffold leaves no directory behind.
func AllocateManagedPath(projectsDir string, name string) (string, error) {
	name, err := cleanName(name)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(projectsDir) == "" {
		return "", ErrPathRequired
	}
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		return "", fmt.Errorf("prepare projects directory: %w", err)
	}

	base := slugify(name)
	if base == "" {
		base = "project"
	}
	for attempt := 1; attempt <= 100; attempt++ {
		candidate := filepath.Join(projectsDir, base)
		if attempt > 1 {
			candidate = filepath.Join(projectsDir, fmt.Sprintf("%s-%d", base, attempt))
		}
		if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("create project directory: too many projects named %q", name)
}

// WorkspaceDirectory returns where new managed projects are created: the
// operator's stored preference, or the platform default.
func WorkspaceDirectory(database *sql.DB) (string, error) {
	dir, err := settings.String(database, WorkspaceRootKey)
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

// Link registers an existing directory without taking ownership of its files.
// Nothing is written into the directory.
func Link(database *sql.DB, rootPath string) (Project, error) {
	root, err := resolveRoot(rootPath)
	if err != nil {
		return Project{}, err
	}

	name := strings.TrimSpace(filepath.Base(root))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return Project{}, ErrNameRequired
	}

	inUse, err := pathInUse(database, root, "")
	if err != nil {
		return Project{}, err
	}
	if inUse {
		return Project{}, ErrPathInUse
	}

	project, err := insert(database, name, root, KindLinked)
	if err != nil {
		return Project{}, err
	}
	if err := activity.Log(database, activity.ScopeProject, project.ID, "Project linked", project.RootPath); err != nil {
		return project, err
	}
	return project, nil
}

// Rename changes a project's display name. Managed directories keep the name
// they were created with.
func Rename(database *sql.DB, id string, name string) (Project, error) {
	name, err := cleanName(name)
	if err != nil {
		return Project{}, err
	}
	project, err := Get(database, id)
	if err != nil {
		return Project{}, err
	}

	if _, err := database.Exec(`UPDATE projects SET name = ? WHERE id = ? AND deleted_at IS NULL`, name, id); err != nil {
		return Project{}, fmt.Errorf("rename project %q: %w", id, err)
	}
	project.Name = name
	if err := activity.Log(database, activity.ScopeProject, id, "Project renamed", name); err != nil {
		return project, err
	}
	return Get(database, id)
}

// ChangeRoot points a linked project at another existing directory. It never
// moves, copies or deletes files.
func ChangeRoot(database *sql.DB, id string, rootPath string) (Project, error) {
	project, err := Get(database, id)
	if err != nil {
		return Project{}, err
	}
	if project.Kind == KindManaged {
		return Project{}, ErrManagedRootFixed
	}

	root, err := resolveRoot(rootPath)
	if err != nil {
		return Project{}, err
	}
	inUse, err := pathInUse(database, root, id)
	if err != nil {
		return Project{}, err
	}
	if inUse {
		return Project{}, ErrPathInUse
	}

	if _, err := database.Exec(`UPDATE projects SET root_path = ? WHERE id = ? AND deleted_at IS NULL`, root, id); err != nil {
		return Project{}, fmt.Errorf("change project root %q: %w", id, err)
	}
	if err := activity.Log(database, activity.ScopeProject, id, "Project root changed", root); err != nil {
		return project, err
	}
	return Get(database, id)
}

// Remove takes a project out of Locafy. The directory and its files stay on
// disk, for managed and linked projects alike.
func Remove(database *sql.DB, id string) error {
	project, err := Get(database, id)
	if err != nil {
		return err
	}
	result, err := database.Exec(`UPDATE projects
		SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("remove project %q: %w", id, err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return activity.Log(database, activity.ScopeProject, id, "Project removed", project.Name)
}

func insert(database *sql.DB, name string, root string, kind string) (Project, error) {
	id := db.NewID()
	_, err := database.Exec(`INSERT INTO projects (id, name, root_path, kind) VALUES (?, ?, ?, ?)`, id, name, root, kind)
	if err != nil {
		if isUniqueViolation(err) {
			return Project{}, ErrPathInUse
		}
		return Project{}, fmt.Errorf("create project %q: %w", name, err)
	}
	return Get(database, id)
}

func scan(row interface{ Scan(...any) error }) (Project, error) {
	var project Project
	err := row.Scan(&project.ID, &project.Name, &project.RootPath, &project.Description,
		&project.Kind, &project.Status, &project.LifecycleState, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, fmt.Errorf("scan project: %w", err)
	}
	return project, nil
}

// isUniqueViolation reports whether err is SQLite's unique-constraint
// failure, which the partial unique index on root_path produces when a
// live project already uses the directory.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrNameRequired
	}
	if len([]rune(name)) > 120 {
		return "", fmt.Errorf("%w: at most 120 characters", ErrNameRequired)
	}
	return name, nil
}

// resolveRoot returns the canonical absolute directory, validated.
func resolveRoot(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", ErrPathRequired
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", path, err)
	}
	abs = filepath.Clean(abs)

	info, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrPathMissing
	}
	if err != nil {
		return "", fmt.Errorf("inspect %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", ErrPathNotDir
	}
	return abs, nil
}

func pathInUse(database *sql.DB, root string, exceptID string) (bool, error) {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM projects
		WHERE deleted_at IS NULL AND lower(root_path) = lower(?) AND id <> ?`, root, exceptID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check project path %q: %w", root, err)
	}
	return count > 0, nil
}

// allocateDir creates a unique slugged directory under parent. The directory
// is created by this call, so a failed insert can safely remove it.
func allocateDir(parent string, name string) (string, error) {
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("prepare projects directory: %w", err)
	}

	base := slugify(name)
	for attempt := 1; attempt <= 100; attempt++ {
		candidate := filepath.Join(parent, base)
		if attempt > 1 {
			candidate = filepath.Join(parent, fmt.Sprintf("%s-%d", base, attempt))
		}
		err := os.Mkdir(candidate, 0o755)
		if err == nil {
			return candidate, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("create project directory: %w", err)
		}
	}
	return "", fmt.Errorf("create project directory: too many projects named %q", name)
}

// slugify reduces a project name to a portable directory name.
func slugify(name string) string {
	var slug strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			slug.WriteRune(r)
			dash = false
		case !dash && slug.Len() > 0:
			slug.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(slug.String(), "-")
}

func writeManifest(root string, project Project) error {
	manifest := struct {
		SchemaVersion int    `json:"schemaVersion"`
		Name          string `json:"name"`
		Description   string `json:"description"`
	}{
		SchemaVersion: 1,
		Name:          project.Name,
		Description:   project.Description,
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project manifest: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(filepath.Join(root, ManifestFile), payload, 0o644); err != nil {
		return fmt.Errorf("write project manifest: %w", err)
	}
	return nil
}
