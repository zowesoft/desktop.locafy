package machine

import (
	"database/sql"
	"path/filepath"
	"testing"

	"locafy.dev/desktop/internal/db"
)

func openMigrated(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func TestRegisterUpsertsSingleRow(t *testing.T) {
	database := openMigrated(t)

	if err := Register(database); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := Register(database); err != nil {
		t.Fatalf("register again: %v", err)
	}

	var hostname, platform, architecture, osVersion string
	err := database.QueryRow(`SELECT hostname, platform, architecture, os_version FROM machine WHERE id = 'local'`).
		Scan(&hostname, &platform, &architecture, &osVersion)
	if err != nil {
		t.Fatalf("read machine record: %v", err)
	}
	if hostname == "" {
		t.Fatal("hostname is empty")
	}
	if platform == "" || architecture == "" {
		t.Fatalf("platform = %q, architecture = %q, want non-empty values", platform, architecture)
	}
	if osVersion == "" {
		t.Fatal("os_version is empty")
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM machine`).Scan(&count); err != nil {
		t.Fatalf("count machine rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("machine rows = %d, want 1", count)
	}
}
