// Package db opens the Locafy SQLite database and applies schema migrations.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens the database at path with the settings the engine relies on:
// WAL journaling, enforced foreign keys, and a busy timeout so concurrent
// service access waits instead of failing immediately.
func Open(path string) (*sql.DB, error) {
	handle, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := handle.Exec(pragma); err != nil {
			handle.Close()
			return nil, fmt.Errorf("apply pragma %q: %w", pragma, err)
		}
	}
	return handle, nil
}
