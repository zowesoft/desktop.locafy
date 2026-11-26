package db

import (
	"database/sql"
	"errors"
	"time"

	sqlite "modernc.org/sqlite"
)

// execer is the part of a database handle a write needs, so the retry can be
// exercised without a live database.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// ExecRetry runs a write, retrying while SQLite reports the database locked or
// the transaction's read snapshot stale. WAL refuses the second kind at once
// instead of waiting, so busy_timeout alone would let a concurrent writer drop
// this one; the retry starts a fresh snapshot and applies it.
func ExecRetry(executor execer, query string, args ...any) (sql.Result, error) {
	delay := time.Millisecond
	for attempt := 0; ; attempt++ {
		result, err := executor.Exec(query, args...)
		if err == nil {
			return result, nil
		}
		if attempt >= 9 || !locked(err) {
			return nil, err
		}
		time.Sleep(delay)
		delay *= 2
	}
}

// locked reports whether err is one SQLite refuses because another connection
// holds the write lock or has committed since this statement read the table.
func locked(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	switch sqliteErr.Code() & 0xff {
	case 5, 6: // SQLITE_BUSY, SQLITE_LOCKED
		return true
	}
	return false
}
