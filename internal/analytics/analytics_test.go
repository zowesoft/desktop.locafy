package analytics

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
)

func newTestDB(t *testing.T) *sql.DB {
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

// logAt records an action and moves it to at, so the trend window can be
// exercised without waiting for days to pass.
func logAt(t *testing.T, database *sql.DB, action string, at time.Time) {
	t.Helper()
	if err := activity.Log(database, activity.ScopeMachine, "", action, ""); err != nil {
		t.Fatalf("log %q: %v", action, err)
	}
	if _, err := database.Exec(`UPDATE activity_logs SET created_at = ?
		WHERE id = (SELECT id FROM activity_logs ORDER BY rowid DESC LIMIT 1)`,
		at.UTC().Format("2006-01-02 15:04:05")); err != nil {
		t.Fatalf("backdate %q: %v", action, err)
	}
}

func TestTrendBucketsEveryDayAndCountsActions(t *testing.T) {
	database := newTestDB(t)
	now := time.Now().UTC()

	logAt(t, database, "Engine started", now)
	logAt(t, database, "Project started", now)
	logAt(t, database, "Project started", now.AddDate(0, 0, -2))

	trend, err := TrendDays(database, 7)
	if err != nil {
		t.Fatalf("trend: %v", err)
	}
	if len(trend.Days) != 7 {
		t.Fatalf("trend has %d days, want 7", len(trend.Days))
	}
	if trend.Days[6].Count != 2 {
		t.Fatalf("today = %d, want 2", trend.Days[6].Count)
	}
	if trend.Days[4].Count != 1 {
		t.Fatalf("two days ago = %d, want 1", trend.Days[4].Count)
	}
	if trend.Days[0].Count != 0 {
		t.Fatalf("the oldest day = %d, want 0", trend.Days[0].Count)
	}
	if trend.Total != 3 {
		t.Fatalf("total = %d, want 3", trend.Total)
	}
	if len(trend.Actions) != 2 || trend.Actions[0].Action != "Project started" || trend.Actions[0].Count != 2 {
		t.Fatalf("actions = %+v, want Project started twice first", trend.Actions)
	}
	if trend.Retained != DefaultRetentionDays {
		t.Fatalf("retained = %d, want the default %d", trend.Retained, DefaultRetentionDays)
	}
}

func TestRecentEventsKeepsMachineScopeAndOrdersNewestFirst(t *testing.T) {
	database := newTestDB(t)
	now := time.Now().UTC()

	logAt(t, database, "Engine started", now.Add(-time.Hour))
	if _, err := database.Exec(`INSERT INTO activity_logs (id, scope_type, scope_id, action, detail, created_at)
		VALUES ('e1', 'project', 'p1', 'Project started', 'started web', ?)`,
		now.Format("2006-01-02 15:04:05")); err != nil {
		t.Fatalf("insert project activity: %v", err)
	}

	events, err := RecentEvents(database, 10)
	if err != nil {
		t.Fatalf("recent events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want the two recorded actions", len(events))
	}
	if events[0].Action != "Project started" || events[0].Scope != "project" || events[0].Detail != "started web" {
		t.Fatalf("first event = %+v, want the newest project action first", events[0])
	}
	if events[1].Action != "Engine started" || events[1].Scope != "machine" {
		t.Fatalf("second event = %+v, want the machine action included", events[1])
	}
	if limited, err := RecentEvents(database, 1); err != nil || len(limited) != 1 {
		t.Fatalf("limited = %d, %v; want one event", len(limited), err)
	}
}

func TestPruneKeepsTheWindowAndReportsWhatItRemoved(t *testing.T) {
	database := newTestDB(t)
	now := time.Now().UTC()

	logAt(t, database, "Recent action", now)
	logAt(t, database, "Old action", now.AddDate(0, 0, -40))

	removed, err := Prune(database, 7)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want the one old row", removed)
	}
	var remaining int
	if err := database.QueryRow(`SELECT count(*) FROM activity_logs`).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining = %d, want 1", remaining)
	}
}

func TestRetentionPreferenceIsStoredValidatedAndApplied(t *testing.T) {
	database := newTestDB(t)
	now := time.Now().UTC()
	logAt(t, database, "Older than the new window", now.AddDate(0, 0, -20))
	logAt(t, database, "Inside the new window", now)

	if err := SetRetention(database, 0); err == nil {
		t.Fatalf("a zero-day retention was accepted")
	}
	if err := SetRetention(database, MaxRetentionDays+1); err == nil {
		t.Fatalf("a retention beyond the ceiling was accepted")
	}
	if err := SetRetention(database, 10); err != nil {
		t.Fatalf("set retention: %v", err)
	}
	days, err := Retention(database)
	if err != nil {
		t.Fatalf("retention: %v", err)
	}
	if days != 10 {
		t.Fatalf("retention = %d, want 10", days)
	}
	var remaining int
	if err := database.QueryRow(`SELECT count(*) FROM activity_logs`).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining = %d, want only the recent row after lowering retention", remaining)
	}
	if removed, err := ApplyRetention(database); err != nil || removed != 0 {
		t.Fatalf("apply retention = %d, %v; want nothing further to prune", removed, err)
	}
}
