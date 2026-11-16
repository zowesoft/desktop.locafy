// Package activity records engine actions ("Project created",
// "PostgreSQL started") as a lightweight local history. Application
// output belongs in log files, not here.
package activity

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"locafy.dev/desktop/internal/db"
)

// Scope identifies what kind of resource an entry refers to.
type Scope string

const (
	ScopeMachine     Scope = "machine"
	ScopeProject     Scope = "project"
	ScopeApplication Scope = "application"
	ScopeService     Scope = "service"
)

// Entry is one recorded engine action.
type Entry struct {
	ID        string
	Scope     Scope
	ScopeID   string
	Action    string
	Detail    string
	CreatedAt time.Time
}

// Log records an engine action. scopeID may be empty for machine-scoped
// entries.
func Log(database *sql.DB, scope Scope, scopeID string, action string, detail string) error {
	_, err := database.Exec(`INSERT INTO activity_logs (id, scope_type, scope_id, action, detail)
		VALUES (?, ?, NULLIF(?, ''), ?, ?)`,
		db.NewID(), scope, scopeID, action, detail)
	if err != nil {
		return fmt.Errorf("log activity: %w", err)
	}
	return nil
}

// ErrNotFound reports an entry that does not exist or is not dismissible.
var ErrNotFound = errors.New("notification not found")

// Recent returns up to limit undismissed entries, newest first. Passing scopes
// narrows the result to those scopes; passing none returns every scope.
func Recent(database *sql.DB, limit int, scopes ...Scope) ([]Entry, error) {
	query := `SELECT id, scope_type, COALESCE(scope_id, ''), action, COALESCE(detail, ''), created_at
		FROM activity_logs
		WHERE dismissed_at IS NULL`
	args := make([]any, 0, len(scopes)+1)
	if len(scopes) > 0 {
		placeholders := make([]string, len(scopes))
		for index, scope := range scopes {
			placeholders[index] = "?"
			args = append(args, string(scope))
		}
		query += " AND scope_type IN (" + strings.Join(placeholders, ", ") + ")"
	}
	query += " ORDER BY created_at DESC, rowid DESC LIMIT ?"
	args = append(args, limit)

	rows, err := database.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list recent activity: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		var createdAt string
		if err := rows.Scan(&entry.ID, &entry.Scope, &entry.ScopeID, &entry.Action, &entry.Detail, &createdAt); err != nil {
			return nil, fmt.Errorf("scan activity entry: %w", err)
		}
		parsed, err := parseTimestamp(createdAt)
		if err != nil {
			return nil, err
		}
		entry.CreatedAt = parsed
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// Dismiss hides one entry from the notification list. The entry stays in the
// history; only the bell stops counting it.
func Dismiss(database *sql.DB, id string) error {
	result, err := database.Exec(`UPDATE activity_logs
		SET dismissed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ? AND dismissed_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("dismiss activity %q: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("dismiss activity %q: %w", id, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// Clear dismisses every undismissed entry and reports how many it hid.
func Clear(database *sql.DB) (int, error) {
	result, err := database.Exec(`UPDATE activity_logs
		SET dismissed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE dismissed_at IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("clear activity: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("clear activity: %w", err)
	}
	return int(affected), nil
}

func parseTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse activity timestamp %q: unsupported format", value)
}
