package status

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
)

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

func TestHealthReportsTheRunningEngine(t *testing.T) {
	database := openDatabase(t)
	service := NewService(database, time.Now().Add(-90*time.Second))

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
}

func TestNotificationsReturnRecentActivityNewestFirst(t *testing.T) {
	database := openDatabase(t)
	service := NewService(database, time.Now())

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
	service := NewService(database, time.Now())

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
