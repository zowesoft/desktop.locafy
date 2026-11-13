// Package activity records engine actions ("Project created",
// "PostgreSQL started") as a lightweight local history. Application
// output belongs in log files, not here.
package activity

import (
	"database/sql"
	"fmt"
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

// Recent returns up to limit entries, newest first.
func Recent(database *sql.DB, limit int) ([]Entry, error) {
	rows, err := database.Query(`SELECT id, scope_type, COALESCE(scope_id, ''), action, COALESCE(detail, ''), created_at
		FROM activity_logs
		ORDER BY created_at DESC, rowid DESC
		LIMIT ?`, limit)
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

func parseTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse activity timestamp %q: unsupported format", value)
}
