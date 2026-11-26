// Package services owns the backing services a project needs: databases,
// caches and similar engines that run as their own process with their own data
// directory. An engine is machine-level; an instance belongs to one project
// (ADR-0008), and it is started natively rather than in a container (ADR-0020).
package services

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/projects"
)

// Engines a service can be created from.
const (
	EnginePostgreSQL = "postgresql"
	EngineMySQL      = "mysql"
)

// Process states, matching the applications vocabulary.
const (
	StatusStopped  = "stopped"
	StatusStarting = "starting"
	StatusRunning  = "running"
	StatusStopping = "stopping"
	StatusError    = "error"
)

// Health states. Empty means unknown: not running yet, or not probed since the
// process started.
const (
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
)

// Defaults for the database and role an instance is initialised with.
const (
	defaultDatabase = "locafy"
	defaultUser     = "locafy"
)

// maxPortCandidates bounds the walk up from an engine's preferred port.
const maxPortCandidates = 100

var (
	ErrUnknownEngine     = errors.New("that service engine is not in the catalogue")
	ErrEngineUnavailable = errors.New("that service engine is not installed on this machine")
	ErrNameRequired      = errors.New("a service needs a name")
	ErrNotFound          = errors.New("service not found")
	ErrServiceBusy       = errors.New("the service is running")
	ErrPortInUse         = errors.New("the service's port is already in use")
	ErrAlreadyRunning    = errors.New("this service is already running")
	ErrNotRunning        = errors.New("this service is not running")
)

// Connection is what an application needs to reach the instance. Authentication
// is loopback trust for a local development instance, so no password exists to
// hand out or leak.
type Connection struct {
	Host        string
	HostPort    string
	Port        int
	Database    string
	User        string
	URL         string
	Environment string
}

// Instance is one configured server: an engine, a data directory and an
// allocated port, owned by a project.
type Instance struct {
	ID              string
	ProjectID       string
	Name            string
	Engine          string
	Version         string
	Port            int
	DataDirectory   string
	DatabaseName    string
	UserName        string
	Status          string
	Error           string
	StartedAt       string
	Health          string
	HealthDetail    string
	HealthCheckedAt string
	PID             *int
	CreatedAt       string
	UpdatedAt       string
	Connection      Connection
}

// Template describes an engine the catalogue offers. Available reports whether
// this machine can run it now; Detail explains why not when it cannot.
type Template struct {
	Engine      string
	Name        string
	Summary     string
	DefaultPort int
	Version     string
	Available   bool
	Location    string
	Detail      string
}

// CreateInput is what attaching a service needs: which engine, and the name the
// operator gave it.
type CreateInput struct {
	Engine string
	Name   string
}

// Removal reports what removing an instance left behind.
type Removal struct {
	Name          string
	DataDirectory string
}

// DataRoot is the directory that holds every service instance's data, keyed by
// instance id so a rename never moves it.
func DataRoot() (string, error) {
	home, err := db.DataHome()
	if err != nil {
		return "", fmt.Errorf("resolve data directory: %w", err)
	}
	return filepath.Join(home, db.AppDirName(), "services"), nil
}

// templates is the catalogue's engine list, whether or not this machine can run
// each one.
func templates() []Template {
	return []Template{postgresTemplate, mysqlTemplate}
}

// Catalogue lists the engines Locafy can attach, with the state of each on this
// machine.
func Catalogue() []Template {
	list := templates()
	for index := range list {
		programs, err := findEngine(list[index].Engine)
		switch {
		case err != nil:
			list[index].Detail = missingEngineDetail(list[index].Engine, err)
		default:
			list[index].Available = true
			list[index].Version = programs.version
			list[index].Location = programs.server
		}
	}
	return list
}

func templateFor(engine string) (Template, bool) {
	for _, template := range Catalogue() {
		if template.Engine == engine {
			return template, true
		}
	}
	return Template{}, false
}

// List returns a project's live services ordered by name.
func List(database *sql.DB, projectID string) ([]Instance, error) {
	rows, err := database.Query(`SELECT `+columns+`
		FROM services WHERE project_id = ? AND deleted_at IS NULL ORDER BY name COLLATE NOCASE`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var list []Instance
	for rows.Next() {
		service, err := scan(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, service)
	}
	return list, rows.Err()
}

// Get returns one live service, or ErrNotFound.
func Get(database *sql.DB, id string) (Instance, error) {
	row := database.QueryRow(`SELECT `+columns+` FROM services WHERE id = ? AND deleted_at IS NULL`, id)
	return scan(row)
}

// Create attaches a service to a project: it allocates the instance's port,
// gives it a directory of its own under the Locafy data home, and records it.
// Nothing is started, and no engine is installed as a side effect.
func Create(database *sql.DB, root string, projectID string, input CreateInput) (Instance, error) {
	if _, err := projects.Get(database, projectID); err != nil {
		return Instance{}, err
	}

	engine := strings.TrimSpace(input.Engine)
	template, known := templateFor(engine)
	if !known {
		return Instance{}, ErrUnknownEngine
	}
	if !template.Available {
		return Instance{}, fmt.Errorf("%w: %s", ErrEngineUnavailable, template.Detail)
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = template.Name
	}
	if len([]rune(name)) > 120 {
		return Instance{}, fmt.Errorf("%w: at most 120 characters", ErrNameRequired)
	}

	port, err := allocatePort(database, template.DefaultPort)
	if err != nil {
		return Instance{}, err
	}

	// MySQL has no trust authentication: a fresh instance's root account is
	// created with an empty password and is only reachable on loopback, which is
	// the same local-development posture PostgreSQL's trust rule gives.
	databaseName, userName := defaultDatabase, defaultUser
	if engine == EngineMySQL {
		userName = "root"
	}

	id := db.NewID()
	dataDirectory := filepath.Join(root, id, "data")
	_, err = database.Exec(`INSERT INTO services
			(id, project_id, name, engine, version, port, data_directory, database_name, user_name)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, projectID, name, engine, template.Version, port, dataDirectory, databaseName, userName)
	if err != nil {
		if isUniqueViolation(err) {
			if strings.Contains(err.Error(), "services.port") {
				return Instance{}, fmt.Errorf("%w: port %d was taken while the service was created", ErrPortInUse, port)
			}
			return Instance{}, fmt.Errorf("a service named %q already exists in this project", name)
		}
		return Instance{}, fmt.Errorf("create service %q: %w", name, err)
	}
	if err := activity.Log(database, activity.ScopeProject, projectID, "Service added", name); err != nil {
		return Instance{}, err
	}
	return Get(database, id)
}

// Remove takes an instance out of Locafy. The record goes; the data directory
// stays on disk, and the caller reports where it is.
func Remove(database *sql.DB, id string) (Removal, error) {
	service, err := Get(database, id)
	if err != nil {
		return Removal{}, err
	}
	_, err = database.Exec(`UPDATE services
		SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return Removal{}, fmt.Errorf("remove service %q: %w", service.Name, err)
	}
	if err := activity.Log(database, activity.ScopeProject, service.ProjectID, "Service removed", service.Name); err != nil {
		return Removal{}, err
	}
	return Removal{Name: service.Name, DataDirectory: service.DataDirectory}, nil
}

const columns = `id, project_id, name, engine, version, port, data_directory, database_name, user_name,
	status, error, started_at, health, health_detail, health_checked_at, pid, created_at, updated_at`

func scan(row interface{ Scan(...any) error }) (Instance, error) {
	var service Instance
	var pid sql.NullInt64
	err := row.Scan(&service.ID, &service.ProjectID, &service.Name, &service.Engine, &service.Version,
		&service.Port, &service.DataDirectory, &service.DatabaseName, &service.UserName, &service.Status,
		&service.Error, &service.StartedAt, &service.Health, &service.HealthDetail, &service.HealthCheckedAt,
		&pid, &service.CreatedAt, &service.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Instance{}, ErrNotFound
		}
		return Instance{}, fmt.Errorf("scan service: %w", err)
	}
	if pid.Valid {
		value := int(pid.Int64)
		service.PID = &value
	}
	service.Connection = connectionFor(service)
	return service, nil
}

func connectionFor(service Instance) Connection {
	hostPort := net.JoinHostPort("127.0.0.1", strconv.Itoa(service.Port))
	scheme := "postgresql"
	if service.Engine == EngineMySQL {
		scheme = "mysql"
	}
	url := fmt.Sprintf("%s://%s@%s/%s", scheme, service.UserName, hostPort, service.DatabaseName)
	return Connection{
		Host:        "127.0.0.1",
		HostPort:    hostPort,
		Port:        service.Port,
		Database:    service.DatabaseName,
		User:        service.UserName,
		URL:         url,
		Environment: "DATABASE_URL=" + url,
	}
}

// allocatePort takes the engine's preferred port, or the next one that neither
// another live instance nor a listener on this machine holds.
func allocatePort(database *sql.DB, preferred int) (int, error) {
	for candidate := preferred; candidate < preferred+maxPortCandidates; candidate++ {
		held, err := portRecorded(database, candidate)
		if err != nil {
			return 0, err
		}
		if held || !portAvailable(candidate) {
			continue
		}
		return candidate, nil
	}
	return 0, fmt.Errorf("no free port between %d and %d", preferred, preferred+maxPortCandidates-1)
}

func portRecorded(database *sql.DB, port int) (bool, error) {
	var count int
	err := database.QueryRow(`SELECT count(*) FROM services WHERE port = ? AND deleted_at IS NULL`, port).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check recorded ports: %w", err)
	}
	return count > 0, nil
}

// portAvailable reports whether Locafy could bind the port. Binding is the only
// question that matters: something listening there means the engine could not.
func portAvailable(port int) bool {
	return bindProbe(port) == nil
}

var bindProbe = listenForProof

func listenForProof(port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	return listener.Close()
}

// instanceDirectory is the folder holding one instance's data and any engine
// scratch files.
func instanceDirectory(dataDirectory string) string {
	return filepath.Dir(dataDirectory)
}

// prepareInstanceDirectory creates the instance folder. The data directory
// itself is created by the engine's initialiser, which requires an empty or
// missing target.
func prepareInstanceDirectory(dataDirectory string) error {
	if err := os.MkdirAll(instanceDirectory(dataDirectory), 0o755); err != nil {
		return fmt.Errorf("create service directory: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
