// Package status reports engine health and recent activity so the desktop
// chrome can show the current state instead of an assumed one.
package status

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/monitor"
	"locafy.dev/desktop/internal/settings"
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
	ID string
	// ScopeID identifies the resource the entry belongs to (a project, for
	// project-scoped actions) so the top bar can open it.
	ScopeID   string
	Scope     string
	Action    string
	Detail    string
	CreatedAt string
	// Severity is "warning" for conditions the user has to act on and "info"
	// for everything recorded in the activity history.
	Severity string
	// Dismissible reports whether the entry can be hidden from the bell.
	// Recorded actions can; a warning derived from the current reading cannot,
	// because dismissing it would only hide a condition that is still true.
	Dismissible bool
	// OpenPath is a folder the entry can reveal in the OS file manager. It is
	// empty for entries with nothing to act on.
	OpenPath string
}

const (
	severityInfo    = "info"
	severityWarning = "warning"

	// diskWarningFreeBytes is the free space below which the engine warns about
	// the data volume. It is an absolute figure because writes fail on free
	// bytes, not on the percentage used, and 5 GiB leaves room to act.
	diskWarningFreeBytes uint64 = 5 << 30
)

// maxNotifications bounds a single page so the top bar cannot ask for the
// whole history by accident.
const maxNotifications = 200

// NotificationPreferences chooses which kinds of notification the bell shows.
// Project covers every scope that describes work on a project, including the
// applications and services that will hang off one.
type NotificationPreferences struct {
	Machine  bool
	Project  bool
	Warnings bool
}

const (
	prefMachineKey  = "notifications.machine"
	prefProjectKey  = "notifications.project"
	prefWarningsKey = "notifications.warning"
)

// resourceSource supplies machine readings. The sampler is the real source;
// tests provide a fixed one so a reading never depends on the test machine.
type resourceSource interface {
	Snapshot() monitor.Snapshot
	Close()
}

// Service exposes engine status to the frontend.
type Service struct {
	database  *sql.DB
	startedAt time.Time
	resources resourceSource
}

// NewService returns a status service for the running engine. Resource usage
// is sampled in the background so status reads stay fast.
func NewService(database *sql.DB, startedAt time.Time) *Service {
	return newService(database, startedAt, monitor.NewSampler(dataVolumePath()))
}

func newService(database *sql.DB, startedAt time.Time, resources resourceSource) *Service {
	return &Service{database: database, startedAt: startedAt, resources: resources}
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

// NotificationPreferences reports which kinds of notification the bell shows.
// Each type is on until the operator turns it off, so an unset preference is
// not a reason to hide anything.
func (s *Service) NotificationPreferences() (NotificationPreferences, error) {
	prefs := NotificationPreferences{Machine: true, Project: true, Warnings: true}

	for _, item := range []struct {
		key    string
		target *bool
	}{
		{prefMachineKey, &prefs.Machine},
		{prefProjectKey, &prefs.Project},
		{prefWarningsKey, &prefs.Warnings},
	} {
		value, err := settings.Bool(s.database, item.key)
		if errors.Is(err, settings.ErrNotFound) {
			continue
		}
		if err != nil {
			return NotificationPreferences{}, err
		}
		*item.target = value
	}

	return prefs, nil
}

// SetNotificationPreferences stores the chosen kinds of notification.
func (s *Service) SetNotificationPreferences(prefs NotificationPreferences) error {
	for _, item := range []struct {
		key   string
		value bool
	}{
		{prefMachineKey, prefs.Machine},
		{prefProjectKey, prefs.Project},
		{prefWarningsKey, prefs.Warnings},
	} {
		if err := settings.SetBool(s.database, item.key, item.value); err != nil {
			return fmt.Errorf("save notification preferences: %w", err)
		}
	}
	return nil
}

// Notifications returns recent engine actions, newest first. The limit is the
// caller's page size: the top bar asks for more as the operator scrolls, so a
// burst of project work cannot bury machine entries permanently.
func (s *Service) Notifications(limit int) ([]Notification, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > maxNotifications {
		limit = maxNotifications
	}

	prefs, err := s.NotificationPreferences()
	if err != nil {
		return nil, err
	}

	scopes := make([]activity.Scope, 0, 4)
	if prefs.Machine {
		scopes = append(scopes, activity.ScopeMachine)
	}
	if prefs.Project {
		scopes = append(scopes, activity.ScopeProject, activity.ScopeApplication, activity.ScopeService)
	}

	var entries []activity.Entry
	if len(scopes) > 0 {
		entries, err = activity.Recent(s.database, limit, scopes...)
		if err != nil {
			return nil, err
		}
	}

	notifications := make([]Notification, 0, len(entries)+1)

	// A condition that needs acting on leads the list: it is current state, not
	// history, and it disappears when the volume recovers because it is derived
	// from the reading rather than recorded as an event.
	if prefs.Warnings {
		usage := s.resources.Snapshot()
		if warning := diskWarning(usage.Disk); warning != nil {
			notifications = append(notifications, *warning)
		}
	}

	for _, entry := range entries {
		notifications = append(notifications, Notification{
			ID:          entry.ID,
			ScopeID:     entry.ScopeID,
			Scope:       string(entry.Scope),
			Action:      entry.Action,
			Detail:      entry.Detail,
			CreatedAt:   entry.CreatedAt.UTC().Format(time.RFC3339),
			Severity:    severityInfo,
			Dismissible: true,
		})
	}

	if len(notifications) > limit {
		notifications = notifications[:limit]
	}

	return notifications, nil
}

// openFolder is a seam for tests: the real implementation hands the path to the
// OS file manager, which is the only way to make a local folder actionable from
// inside a webview.
var openFolder = func(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// OpenDataFolder reveals the folder holding the engine's data — the volume the
// disk warning describes — in the OS file manager.
func (s *Service) OpenDataFolder() error {
	path := dataVolumePath()
	if err := openFolder(path); err != nil {
		return fmt.Errorf("open data folder %q: %w", path, err)
	}
	return nil
}

// Dismiss hides one recorded notification. The activity entry stays in the
// history; only the bell stops counting it.
func (s *Service) Dismiss(id string) error {
	return activity.Dismiss(s.database, id)
}

// ClearNotifications hides every recorded notification and reports how many
// were hidden.
func (s *Service) ClearNotifications() (int, error) {
	return activity.Clear(s.database)
}

// diskWarning reports the data volume when free space has fallen far enough
// that a write could fail.
func diskWarning(usage monitor.Usage) *Notification {
	if usage.TotalBytes == 0 || usage.UsedBytes >= usage.TotalBytes {
		return nil
	}

	free := usage.TotalBytes - usage.UsedBytes
	if free >= diskWarningFreeBytes {
		return nil
	}

	return &Notification{
		ID:        "disk-space",
		Scope:     string(activity.ScopeMachine),
		Action:    "Low disk space",
		Detail:    fmt.Sprintf("%s free of %s on the data volume", formatBytes(free), formatBytes(usage.TotalBytes)),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Severity:  severityWarning,
		OpenPath:  dataVolumePath(),
	}
}

// formatBytes renders a byte count for a reader, not for a machine.
func formatBytes(bytes uint64) string {
	const gibibyte = 1 << 30
	if bytes >= gibibyte {
		return fmt.Sprintf("%.1f GB", float64(bytes)/gibibyte)
	}

	return fmt.Sprintf("%.0f MB", float64(bytes)/(1<<20))
}
