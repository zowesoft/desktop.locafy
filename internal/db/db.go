// Package db opens the Locafy SQLite database and applies schema migrations.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens the database at path with the settings the engine relies on:
// WAL journaling, enforced foreign keys, and a busy timeout so concurrent
// service access waits instead of failing immediately. They travel in the DSN
// because the driver applies those to every connection the pool opens; a
// statement would only reach the first one and leave later connections without
// the timeout or the foreign-key enforcement.
func Open(path string) (*sql.DB, error) {
	handle, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=1&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// The driver opens the file lazily, so a path that cannot be opened would
	// otherwise surface on the first query instead of here.
	if err := handle.Ping(); err != nil {
		handle.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	return handle, nil
}
