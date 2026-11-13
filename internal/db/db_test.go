package db

import (
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
	if _, err := database.Exec(`INSERT INTO settings (key, value) VALUES ('test', 'value')`); err != nil {
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

	for i := 0; i < 3; i++ {
		if err := Migrate(database); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 1 {
		t.Fatalf("schema_migrations rows = %d, want 1", count)
	}
}

func TestOpenRejectsMissingDirectory(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing", "locafy.db")); err == nil {
		t.Fatal("open in missing directory succeeded, want error")
	}
}
