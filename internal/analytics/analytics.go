// Package analytics reads the engine's recorded action history as numbers: how
// much happened on each day, which actions dominate, and how long the history
// is kept. Nothing here samples the machine — these are facts Locafy already
// wrote down.
package analytics

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"locafy.dev/desktop/internal/settings"
)

// RetentionKey stores how many days of activity history are kept.
const RetentionKey = "analytics.retention_days"

// DefaultRetentionDays bounds a history the engine has never been told to trim.
const DefaultRetentionDays = 90

// MaxRetentionDays keeps a preference typo from deleting, or pinning, years of
// history in one step.
const MaxRetentionDays = 3650

// Day is one calendar day of recorded actions, in UTC.
type Day struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

// Action is how often one recorded action appears in the window.
type Action struct {
	Action string `json:"action"`
	Count  int    `json:"count"`
}

// Trend is the activity history over the requested window. It is machine-wide:
// every project, application and service the engine recorded.
type Trend struct {
	Days     []Day    `json:"days"`
	Actions  []Action `json:"actions"`
	Total    int      `json:"total"`
	Retained int      `json:"retainedDays"`
}

// Event is one recorded action, for the timeline view.
type Event struct {
	ID      string `json:"id"`
	Scope   string `json:"scope"`
	ScopeID string `json:"scopeId"`
	Action  string `json:"action"`
	Detail  string `json:"detail"`
	At      string `json:"at"`
}

// RecentEvents returns the newest recorded actions, machine scope included, so a
// timeline can show what the engine did rather than only project actions.
func RecentEvents(database *sql.DB, limit int) ([]Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := database.Query(`SELECT id, scope_type, coalesce(scope_id, ''), action, coalesce(detail, ''), created_at
		FROM activity_logs
		ORDER BY created_at DESC, rowid DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read recent activity: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.Scope, &event.ScopeID, &event.Action, &event.Detail, &event.At); err != nil {
			return nil, fmt.Errorf("scan recent activity: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// TrendDays returns the recorded actions per day for the last days days,
// oldest first, filling days with no activity with zero so a gap is visible as
// a gap rather than a shorter chart.
func TrendDays(database *sql.DB, days int) (Trend, error) {
	if days < 1 {
		days = 7
	}
	if days > 366 {
		days = 366
	}
	retained, err := Retention(database)
	if err != nil {
		return Trend{}, err
	}
	trend := Trend{Days: make([]Day, 0, days), Retained: retained}

	since := time.Now().UTC().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := database.Query(`SELECT date(created_at) AS day, count(*)
		FROM activity_logs
		WHERE date(created_at) >= ?
		GROUP BY day`, since)
	if err != nil {
		return Trend{}, fmt.Errorf("read activity trend: %w", err)
	}
	counts := map[string]int{}
	for rows.Next() {
		var day string
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			rows.Close()
			return Trend{}, fmt.Errorf("scan activity trend: %w", err)
		}
		counts[day] = count
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Trend{}, fmt.Errorf("read activity trend: %w", err)
	}

	today := time.Now().UTC()
	for offset := days - 1; offset >= 0; offset-- {
		key := today.AddDate(0, 0, -offset).Format("2006-01-02")
		count := counts[key]
		trend.Days = append(trend.Days, Day{Day: key, Count: count})
		trend.Total += count
	}

	actionRows, err := database.Query(`SELECT action, count(*) AS total
		FROM activity_logs
		WHERE date(created_at) >= ?
		GROUP BY action
		ORDER BY total DESC, action
		LIMIT 10`, since)
	if err != nil {
		return Trend{}, fmt.Errorf("read action mix: %w", err)
	}
	defer actionRows.Close()
	for actionRows.Next() {
		var action Action
		if err := actionRows.Scan(&action.Action, &action.Count); err != nil {
			return Trend{}, fmt.Errorf("scan action mix: %w", err)
		}
		trend.Actions = append(trend.Actions, action)
	}
	return trend, actionRows.Err()
}

// Retention returns how many days of history are kept: the stored preference,
// or the default when the operator has never set one.
func Retention(database *sql.DB) (int, error) {
	value, err := settings.String(database, RetentionKey)
	if errors.Is(err, settings.ErrNotFound) {
		return DefaultRetentionDays, nil
	}
	if err != nil {
		return 0, err
	}
	days, err := strconv.Atoi(value)
	if err != nil || days < 1 || days > MaxRetentionDays {
		return DefaultRetentionDays, nil
	}
	return days, nil
}

// SetRetention stores the preference and prunes immediately, so lowering it
// takes effect now rather than at the next start.
func SetRetention(database *sql.DB, days int) error {
	if days < 1 || days > MaxRetentionDays {
		return fmt.Errorf("keep between 1 and %d days of history, got %d", MaxRetentionDays, days)
	}
	if err := settings.SetString(database, RetentionKey, strconv.Itoa(days)); err != nil {
		return err
	}
	_, err := Prune(database, days)
	return err
}

// Prune deletes recorded actions older than keepDays and reports how many rows
// it removed. History is bounded on purpose: the table is a local debugging
// aid, not an archive.
func Prune(database *sql.DB, keepDays int) (int, error) {
	if keepDays < 1 {
		keepDays = DefaultRetentionDays
	}
	result, err := database.Exec(`DELETE FROM activity_logs
		WHERE created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", keepDays))
	if err != nil {
		return 0, fmt.Errorf("prune activity history: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// ApplyRetention prunes the history to the stored preference. The engine calls
// it once at startup so a machine that is rarely restarted still trims.
func ApplyRetention(database *sql.DB) (int, error) {
	days, err := Retention(database)
	if err != nil {
		return 0, err
	}
	return Prune(database, days)
}
