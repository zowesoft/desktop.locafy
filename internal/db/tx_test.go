package db

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestWithTxCommitsOnSuccess(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	err = WithTx(database, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO settings (key, value, value_type) VALUES ('a', '1', 'string')`)
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM settings WHERE key = 'a'`).Scan(&count); err != nil {
		t.Fatalf("query committed row: %v", err)
	}
	if count != 1 {
		t.Fatalf("committed rows = %d, want 1", count)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()
	if err := Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	wantErr := errors.New("boom")
	err = WithTx(database, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO settings (key, value, value_type) VALUES ('b', '1', 'string')`); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithTx error = %v, want %v", err, wantErr)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM settings WHERE key = 'b'`).Scan(&count); err != nil {
		t.Fatalf("query rolled-back row: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows after rollback = %d, want 0", count)
	}
}
