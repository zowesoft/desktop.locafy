package status

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/monitor"
)

// fixedResources is a machine reading that does not depend on the test host.
type fixedResources struct {
	snapshot monitor.Snapshot
}

func (f fixedResources) Snapshot() monitor.Snapshot { return f.snapshot }
func (f fixedResources) Close()                     {}

var healthyVolume = monitor.Snapshot{
	CPUPercent: 12.5,
	Memory:     monitor.Usage{UsedBytes: 2 << 30, TotalBytes: 8 << 30, Percent: 25},
	Disk:       monitor.Usage{UsedBytes: 100 << 30, TotalBytes: 500 << 30, Percent: 20},
	CapturedAt: time.Now(),
}

func openDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func newTestService(t *testing.T, database *sql.DB, resources monitor.Snapshot) *Service {
	t.Helper()

	service := newService(database, time.Now(), fixedResources{snapshot: resources})
	t.Cleanup(service.Close)

	return service
}

func TestHealthReportsTheRunningEngine(t *testing.T) {
	database := openDatabase(t)
	service := newService(database, time.Now().Add(-90*time.Second), fixedResources{snapshot: healthyVolume})

	health := service.Health()

	if !health.Healthy || !health.DatabaseOK {
		t.Fatalf("health = %+v, want a healthy engine with a reachable database", health)
	}
	if health.Summary != "All systems operational" {
		t.Fatalf("summary = %q, want the operational message", health.Summary)
	}
	if health.Platform != runtime.GOOS {
		t.Fatalf("platform = %q, want %q", health.Platform, runtime.GOOS)
	}
	if health.Hostname == "" {
		t.Fatal("hostname is empty, want the current host")
	}
	if health.DatabasePath == "" {
		t.Fatal("database path is empty, want the resolved path")
	}
	if health.UptimeSeconds < 90 {
		t.Fatalf("uptime = %d, want at least 90 seconds", health.UptimeSeconds)
	}
	if _, err := time.Parse(time.RFC3339, health.StartedAt); err != nil {
		t.Fatalf("started at = %q, want an RFC 3339 timestamp: %v", health.StartedAt, err)
	}
	if health.Memory.TotalBytes == 0 {
		t.Error("memory total is zero, want the sampled reading")
	}
	if health.Memory.UsedBytes > health.Memory.TotalBytes {
		t.Errorf("memory used %d exceeds total %d", health.Memory.UsedBytes, health.Memory.TotalBytes)
	}
	if health.Disk.TotalBytes == 0 {
		t.Error("disk total is zero, want the volume holding the database")
	}
	if health.CPUPercent != healthyVolume.CPUPercent {
		t.Errorf("cpu = %v, want the sampled %v", health.CPUPercent, healthyVolume.CPUPercent)
	}
}

func TestNotificationsReturnRecentActivityNewestFirst(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	for _, action := range []string{"Engine started", "Machine registered"} {
		if err := activity.Log(database, activity.ScopeMachine, "", action, ""); err != nil {
			t.Fatalf("log activity: %v", err)
		}
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 2 {
		t.Fatalf("got %d notifications, want 2", len(notifications))
	}
	if notifications[0].Action != "Machine registered" {
		t.Fatalf("first notification = %q, want the newest entry", notifications[0].Action)
	}
	if _, err := time.Parse(time.RFC3339, notifications[0].CreatedAt); err != nil {
		t.Fatalf("created at = %q, want an RFC 3339 timestamp: %v", notifications[0].CreatedAt, err)
	}
}

func TestNotificationsRespectTheLimit(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	for _, action := range []string{"first", "second", "third"} {
		if err := activity.Log(database, activity.ScopeMachine, "", action, ""); err != nil {
			t.Fatalf("log activity: %v", err)
		}
	}

	notifications, err := service.Notifications(2)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 2 {
		t.Fatalf("got %d notifications, want 2", len(notifications))
	}

	fallback, err := service.Notifications(0)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(fallback) != 3 {
		t.Fatalf("got %d notifications for a zero limit, want the default of 10 capped by 3 entries", len(fallback))
	}
}

func TestLowDiskSpaceLeadsTheNotifications(t *testing.T) {
	database := openDatabase(t)
	nearlyFull := healthyVolume
	nearlyFull.Disk = monitor.Usage{UsedBytes: 499 << 30, TotalBytes: 500 << 30, Percent: 99.8}
	service := newTestService(t, database, nearlyFull)

	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 2 {
		t.Fatalf("got %d notifications, want the warning and the logged entry", len(notifications))
	}

	warning := notifications[0]
	if warning.Severity != severityWarning {
		t.Errorf("severity = %q, want %q", warning.Severity, severityWarning)
	}
	if warning.Action != "Low disk space" {
		t.Errorf("action = %q, want the disk warning", warning.Action)
	}
	if !strings.Contains(warning.Detail, "1.0 GB free of 500.0 GB") {
		t.Errorf("detail = %q, want the free and total space", warning.Detail)
	}
}

func TestLowDiskSpaceIsCountedAgainstTheLimit(t *testing.T) {
	database := openDatabase(t)
	nearlyFull := healthyVolume
	nearlyFull.Disk = monitor.Usage{UsedBytes: 499 << 30, TotalBytes: 500 << 30, Percent: 99.8}
	service := newTestService(t, database, nearlyFull)

	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	notifications, err := service.Notifications(1)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("got %d notifications, want 1", len(notifications))
	}
	if notifications[0].Severity != severityWarning {
		t.Fatalf("kept %q, want the warning to survive the limit", notifications[0].Action)
	}
}

func TestDiskWarningFiresBelowTheThreshold(t *testing.T) {
	warning := diskWarning(monitor.Usage{
		UsedBytes:  (500 << 30) - (2 << 30),
		TotalBytes: 500 << 30,
	})

	if warning == nil {
		t.Fatal("no warning for 2 GB free on a 500 GB volume")
	}
	if warning.Severity != severityWarning {
		t.Errorf("severity = %q, want %q", warning.Severity, severityWarning)
	}
	if !strings.Contains(warning.Detail, "2.0 GB free of 500.0 GB") {
		t.Errorf("detail = %q, want the free and total space", warning.Detail)
	}
	if _, err := time.Parse(time.RFC3339, warning.CreatedAt); err != nil {
		t.Errorf("created at = %q, want an RFC 3339 timestamp: %v", warning.CreatedAt, err)
	}
}

func TestDiskWarningStaysSilentAboveTheThreshold(t *testing.T) {
	if warning := diskWarning(healthyVolume.Disk); warning != nil {
		t.Fatalf("warning = %+v, want none for 400 GB free", warning)
	}
}

func TestDiskWarningIgnoresAMissingReading(t *testing.T) {
	if warning := diskWarning(monitor.Usage{}); warning != nil {
		t.Fatalf("warning = %+v, want none when the volume could not be read", warning)
	}
}

func TestActivityIsMarkedInformational(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("got %d notifications, want 1", len(notifications))
	}
	if notifications[0].Severity != severityInfo {
		t.Errorf("severity = %q, want %q", notifications[0].Severity, severityInfo)
	}
}
