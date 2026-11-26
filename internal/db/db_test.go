package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateCreatesSchemaAndPersistsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locafy.db")

	database, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO settings (key, value, value_type) VALUES ('test', 'value', 'string')`); err != nil {
		t.Fatalf("insert setting: %v", err)
	}
	database.Close()

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	if err := Migrate(reopened); err != nil {
		t.Fatalf("migrate after reopen: %v", err)
	}

	var value string
	if err := reopened.QueryRow(`SELECT value FROM settings WHERE key = 'test'`).Scan(&value); err != nil {
		t.Fatalf("read persisted setting: %v", err)
	}
	if value != "value" {
		t.Fatalf("persisted setting = %q, want %q", value, "value")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := Migrate(database); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != len(migrations) {
		t.Fatalf("schema_migrations rows = %d, want %d", count, len(migrations))
	}
}

func TestOpenRejectsMissingDirectory(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing", "locafy.db")); err == nil {
		t.Fatal("open in missing directory succeeded, want error")
	}
}

// The pool opens connections as it needs them, so a setting applied with a
// statement on the first connection leaves the rest without it. Under
// concurrent writes that turns into immediate "database is locked" refusals
// that busy_timeout is meant to absorb.
func TestOpenAppliesSettingsToEveryConnection(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	database.SetMaxOpenConns(2)

	ctx := context.Background()
	first, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("first connection: %v", err)
	}
	defer first.Close()
	second, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("second connection: %v", err)
	}
	defer second.Close()

	for name, connection := range map[string]*sql.Conn{"first": first, "second": second} {
		var busyTimeout int
		if err := connection.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("%s: busy_timeout: %v", name, err)
		}
		if busyTimeout != 5000 {
			t.Errorf("%s: busy_timeout = %d, want 5000", name, busyTimeout)
		}

		var foreignKeys int
		if err := connection.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("%s: foreign_keys: %v", name, err)
		}
		if foreignKeys != 1 {
			t.Errorf("%s: foreign_keys = %d, want 1", name, foreignKeys)
		}

		var journalMode string
		if err := connection.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
			t.Fatalf("%s: journal_mode: %v", name, err)
		}
		if journalMode != "wal" {
			t.Errorf("%s: journal_mode = %q, want wal", name, journalMode)
		}
	}
}
