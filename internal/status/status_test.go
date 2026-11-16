package status

import (
	"database/sql"
	"errors"
	"os"
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

func TestEntriesAreUnreadUntilTheBellIsOpened(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	for _, action := range []string{"Engine started", "Machine registered"} {
		if err := activity.Log(database, activity.ScopeMachine, "", action, ""); err != nil {
			t.Fatalf("log activity: %v", err)
		}
	}

	unread, err := service.UnreadNotifications()
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if unread != 2 {
		t.Fatalf("unread = %d, want 2 before the bell is ever opened", unread)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if notifications[0].Seen {
		t.Fatal("an entry is marked seen before the bell was opened")
	}

	if err := service.MarkNotificationsSeen(); err != nil {
		t.Fatalf("mark seen: %v", err)
	}

	unread, err = service.UnreadNotifications()
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if unread != 0 {
		t.Fatalf("unread = %d, want 0 after opening the bell", unread)
	}

	notifications, err = service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	for _, notification := range notifications {
		if !notification.Seen {
			t.Fatalf("entry %q is still unseen after opening the bell", notification.Action)
		}
	}
}

func TestNewActivityAfterOpeningIsUnreadAgain(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}
	if err := service.MarkNotificationsSeen(); err != nil {
		t.Fatalf("mark seen: %v", err)
	}

	// Recorded a moment later, so it sits after the last-seen mark.
	time.Sleep(1100 * time.Millisecond)
	if err := activity.Log(database, activity.ScopeProject, "p1", "Project created", "Etugora"); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	unread, err := service.UnreadNotifications()
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if unread != 1 {
		t.Fatalf("unread = %d, want only the entry recorded after the bell was opened", unread)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if notifications[0].Seen || !notifications[1].Seen {
		t.Fatalf("seen flags = %v/%v, want the newest unseen and the older one seen",
			notifications[0].Seen, notifications[1].Seen)
	}
}

func TestUnreadCountIgnoresDismissedAndDisabledTypes(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	if err := activity.Log(database, activity.ScopeProject, "p1", "Project created", "Etugora"); err != nil {
		t.Fatalf("log activity: %v", err)
	}
	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	for _, notification := range notifications {
		if notification.Action == "Project created" {
			if err := service.Dismiss(notification.ID); err != nil {
				t.Fatalf("dismiss: %v", err)
			}
		}
	}

	unread, err := service.UnreadNotifications()
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if unread != 1 {
		t.Fatalf("unread = %d, want only the undismissed machine entry", unread)
	}

	if err := service.SetNotificationPreferences(NotificationPreferences{Machine: false, Project: true, Warnings: true}); err != nil {
		t.Fatalf("save preferences: %v", err)
	}

	unread, err = service.UnreadNotifications()
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if unread != 0 {
		t.Fatalf("unread = %d, want 0 with machine events turned off", unread)
	}
}

func TestWarningsAreCurrentStateNotUnreadNews(t *testing.T) {
	database := openDatabase(t)
	lowVolume := monitor.Snapshot{
		CPUPercent: healthyVolume.CPUPercent,
		Memory:     healthyVolume.Memory,
		Disk:       monitor.Usage{UsedBytes: 499 << 30, TotalBytes: 500 << 30, Percent: 99},
		CapturedAt: time.Now(),
	}
	service := newTestService(t, database, lowVolume)

	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	unread, err := service.UnreadNotifications()
	if err != nil {
		t.Fatalf("unread: %v", err)
	}
	if unread != 1 {
		t.Fatalf("unread = %d, want the recorded entry alone", unread)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if !notifications[0].Seen {
		t.Fatal("the warning counts as unseen news")
	}
}

func TestNotificationPreferencesStartFullyOn(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	prefs, err := service.NotificationPreferences()
	if err != nil {
		t.Fatalf("preferences: %v", err)
	}
	if !prefs.Machine || !prefs.Project || !prefs.Warnings {
		t.Fatalf("preferences = %+v, want every type enabled by default", prefs)
	}
}

func TestTurningOffProjectActivityHidesOnlyProjectEntries(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	if err := activity.Log(database, activity.ScopeProject, "p1", "Project created", "Etugora"); err != nil {
		t.Fatalf("log activity: %v", err)
	}
	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	if err := service.SetNotificationPreferences(NotificationPreferences{Machine: true, Project: false, Warnings: true}); err != nil {
		t.Fatalf("save preferences: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Action != "Engine started" {
		t.Fatalf("notifications = %+v, want only the machine entry", notifications)
	}
}

func TestTurningOffWarningsHidesTheDiskWarning(t *testing.T) {
	database := openDatabase(t)
	lowVolume := monitor.Snapshot{
		CPUPercent: healthyVolume.CPUPercent,
		Memory:     healthyVolume.Memory,
		Disk:       monitor.Usage{UsedBytes: 499 << 30, TotalBytes: 500 << 30, Percent: 99},
		CapturedAt: time.Now(),
	}
	service := newTestService(t, database, lowVolume)

	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}
	if err := service.SetNotificationPreferences(NotificationPreferences{Machine: true, Project: true, Warnings: false}); err != nil {
		t.Fatalf("save preferences: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Severity != severityInfo {
		t.Fatalf("notifications = %+v, want the warning gone and the entry kept", notifications)
	}
}

func TestNotificationsCanBeTurnedOffEntirely(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	if err := activity.Log(database, activity.ScopeProject, "p1", "Project created", "Etugora"); err != nil {
		t.Fatalf("log activity: %v", err)
	}
	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}
	if err := service.SetNotificationPreferences(NotificationPreferences{}); err != nil {
		t.Fatalf("save preferences: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 0 {
		t.Fatalf("notifications = %+v, want none", notifications)
	}

	stored, err := service.NotificationPreferences()
	if err != nil {
		t.Fatalf("preferences: %v", err)
	}
	if stored.Machine || stored.Project || stored.Warnings {
		t.Fatalf("stored preferences = %+v, want every type off", stored)
	}
}

func TestDiskWarningOffersTheFolderToOpen(t *testing.T) {
	database := openDatabase(t)
	lowVolume := monitor.Snapshot{
		CPUPercent: healthyVolume.CPUPercent,
		Memory:     healthyVolume.Memory,
		Disk:       monitor.Usage{UsedBytes: 499 << 30, TotalBytes: 500 << 30, Percent: 99},
		CapturedAt: time.Now(),
	}
	service := newTestService(t, database, lowVolume)

	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 2 {
		t.Fatalf("got %d notifications, want the warning and the entry", len(notifications))
	}
	if notifications[0].OpenPath == "" {
		t.Fatal("warning has no folder to open, so it cannot be acted on")
	}
	if notifications[1].OpenPath != "" {
		t.Fatalf("entry open path = %q, want none", notifications[1].OpenPath)
	}
}

func TestOpenDataFolderRevealsTheDataDirectory(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	original := openFolder
	t.Cleanup(func() { openFolder = original })

	var opened string
	openFolder = func(path string) error {
		opened = path
		return nil
	}

	if err := service.OpenDataFolder(); err != nil {
		t.Fatalf("open data folder: %v", err)
	}
	if opened != dataVolumePath() {
		t.Fatalf("opened %q, want the data volume %q", opened, dataVolumePath())
	}
	if info, err := os.Stat(opened); err != nil || !info.IsDir() {
		t.Fatalf("opened path %q is not an existing folder: %v", opened, err)
	}

	openFolder = func(string) error { return errFileManagerUnavailable }
	if err := service.OpenDataFolder(); !errors.Is(err, errFileManagerUnavailable) {
		t.Fatalf("err = %v, want the file manager failure reported", err)
	}
}

var errFileManagerUnavailable = errors.New("file manager unavailable")

func TestDismissHidesOneNotificationAndKeepsTheHistory(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	if err := activity.Log(database, activity.ScopeProject, "p1", "Project created", "Etugora"); err != nil {
		t.Fatalf("log activity: %v", err)
	}
	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	var id string
	if err := database.QueryRow(`SELECT id FROM activity_logs WHERE action = 'Project created'`).Scan(&id); err != nil {
		t.Fatalf("read activity id: %v", err)
	}

	if err := service.Dismiss(id); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Action != "Engine started" {
		t.Fatalf("notifications = %+v, want only the engine entry", notifications)
	}

	// Dismissing is a view concern: the record itself survives for diagnostics.
	var stored int
	if err := database.QueryRow(`SELECT COUNT(*) FROM activity_logs WHERE id = ?`, id).Scan(&stored); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if stored != 1 {
		t.Fatalf("stored entries = %d, want the dismissed entry kept", stored)
	}

	if err := service.Dismiss(id); err == nil {
		t.Fatal("dismissing twice should report that there is nothing left to hide")
	}
}

func TestClearNotificationsHidesEverythingAndReportsTheCount(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	for _, action := range []string{"first", "second", "third"} {
		if err := activity.Log(database, activity.ScopeMachine, "", action, ""); err != nil {
			t.Fatalf("log activity: %v", err)
		}
	}

	hidden, err := service.ClearNotifications()
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if hidden != 3 {
		t.Fatalf("hidden = %d, want 3", hidden)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 0 {
		t.Fatalf("notifications = %+v, want none", notifications)
	}
}

func TestOnlyRecordedEntriesAreDismissible(t *testing.T) {
	database := openDatabase(t)
	// Free space below the warning threshold, so the derived entry leads the list.
	lowVolume := monitor.Snapshot{
		CPUPercent: healthyVolume.CPUPercent,
		Memory:     healthyVolume.Memory,
		Disk:       monitor.Usage{UsedBytes: 499 << 30, TotalBytes: 500 << 30, Percent: 99},
		CapturedAt: time.Now(),
	}
	service := newTestService(t, database, lowVolume)

	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 2 {
		t.Fatalf("got %d notifications, want the warning and the entry", len(notifications))
	}
	if notifications[0].Severity != severityWarning || notifications[0].Dismissible {
		t.Fatalf("warning = %+v, want an undismissible warning", notifications[0])
	}
	if notifications[1].Severity != severityInfo || !notifications[1].Dismissible {
		t.Fatalf("entry = %+v, want a dismissible info entry", notifications[1])
	}
}

func TestNotificationsAcceptAPageLargerThanTheOldCeiling(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	for index := 0; index < 5; index++ {
		if err := activity.Log(database, activity.ScopeProject, "project", "Project created", "Etugora"); err != nil {
			t.Fatalf("log activity: %v", err)
		}
	}

	notifications, err := service.Notifications(1000)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 5 {
		t.Fatalf("got %d notifications, want all 5", len(notifications))
	}
}

func TestNotificationsCarryTheProjectTheyBelongTo(t *testing.T) {
	database := openDatabase(t)
	service := newTestService(t, database, healthyVolume)

	if err := activity.Log(database, activity.ScopeProject, "project-42", "Project created", "Etugora"); err != nil {
		t.Fatalf("log activity: %v", err)
	}
	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log activity: %v", err)
	}

	notifications, err := service.Notifications(10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	if len(notifications) != 2 {
		t.Fatalf("got %d notifications, want 2", len(notifications))
	}

	// Both entries land in the same second, so the list is ordered by write
	// order rather than by action; find them by scope instead of by index.
	byScope := make(map[string]Notification, len(notifications))
	for _, notification := range notifications {
		byScope[notification.Scope] = notification
	}
	if entry := byScope["project"]; entry.ScopeID != "project-42" {
		t.Fatalf("project scope id = %q, want project-42", entry.ScopeID)
	}
	if entry := byScope["machine"]; entry.ScopeID != "" {
		t.Fatalf("machine scope id = %q, want empty", entry.ScopeID)
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
