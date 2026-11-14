// Package status reports engine health and recent activity so the desktop
// chrome can show the current state instead of an assumed one.
package status

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/monitor"
)

// Health is the engine and machine state shown in the top bar.
type Health struct {
	Name          string
	Healthy       bool
	Summary       string
	Hostname      string
	Platform      string
	Architecture  string
	DatabasePath  string
	DatabaseOK    bool
	StartedAt     string
	UptimeSeconds int
	CPUPercent    float64
	Memory        monitor.Usage
	Disk          monitor.Usage
}

// Notification is one recent engine action surfaced in the top bar. Entries
// come from the activity history, so the list reflects recorded events rather
// than a separate notification store.
type Notification struct {
	ID        string
	Scope     string
	Action    string
	Detail    string
	CreatedAt string
}

// Service exposes engine status to the frontend.
type Service struct {
	database  *sql.DB
	startedAt time.Time
	resources *monitor.Sampler
}

// NewService returns a status service for the running engine. Resource usage
// is sampled in the background so status reads stay fast.
func NewService(database *sql.DB, startedAt time.Time) *Service {
	return &Service{
		database:  database,
		startedAt: startedAt,
		resources: monitor.NewSampler(dataVolumePath()),
	}
}

// Close releases the background resource sampler.
func (s *Service) Close() {
	s.resources.Close()
}

// dataVolumePath is the directory whose volume the disk reading describes: the
// drive holding the engine's data, which is the one Locafy fills.
func dataVolumePath() string {
	if path, err := db.DefaultPath(); err == nil {
		return filepath.Dir(path)
	}

	return "."
}

// Health reports the engine state, checking the database rather than assuming
// it is reachable.
func (s *Service) Health() Health {
	health := Health{
		Name:          "Locafy",
		Platform:      runtime.GOOS,
		Architecture:  runtime.GOARCH,
		StartedAt:     s.startedAt.UTC().Format(time.RFC3339),
		UptimeSeconds: int(time.Since(s.startedAt).Seconds()),
	}

	if hostname, err := os.Hostname(); err == nil {
		health.Hostname = hostname
	}
	if path, err := db.DefaultPath(); err == nil {
		health.DatabasePath = path
	}

	health.DatabaseOK = s.database.Ping() == nil
	health.Healthy = health.DatabaseOK
	if health.Healthy {
		health.Summary = "All systems operational"
	} else {
		health.Summary = "Database unavailable"
	}

	usage := s.resources.Snapshot()
	health.CPUPercent = usage.CPUPercent
	health.Memory = usage.Memory
	health.Disk = usage.Disk

	return health
}

// Notifications returns recent engine actions, newest first.
func (s *Service) Notifications(limit int) ([]Notification, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	entries, err := activity.Recent(s.database, limit)
	if err != nil {
		return nil, err
	}

	notifications := make([]Notification, 0, len(entries))
	for _, entry := range entries {
		notifications = append(notifications, Notification{
			ID:        entry.ID,
			Scope:     string(entry.Scope),
			Action:    entry.Action,
			Detail:    entry.Detail,
			CreatedAt: entry.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	return notifications, nil
}
