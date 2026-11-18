package activity

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

func TestLogAndRecent(t *testing.T) {
	database := openMigrated(t)

	if err := Log(database, ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log machine entry: %v", err)
	}
	if err := Log(database, ScopeProject, "proj-1", "Project created", "Etugora"); err != nil {
		t.Fatalf("log project entry: %v", err)
	}

	entries, err := Recent(database, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Action != "Project created" {
		t.Fatalf("newest entry = %q, want %q", entries[0].Action, "Project created")
	}
	if entries[0].Scope != ScopeProject || entries[0].ScopeID != "proj-1" {
		t.Fatalf("scope = %q/%q, want project/proj-1", entries[0].Scope, entries[0].ScopeID)
	}
	if entries[1].Scope != ScopeMachine || entries[1].ScopeID != "" {
		t.Fatalf("scope = %q/%q, want machine/empty", entries[1].Scope, entries[1].ScopeID)
	}
	if entries[0].CreatedAt.IsZero() {
		t.Fatal("created_at not parsed")
	}
}

func TestRecentLimit(t *testing.T) {
	database := openMigrated(t)

	for i := 0; i < 5; i++ {
		if err := Log(database, ScopeMachine, "", "tick", ""); err != nil {
			t.Fatalf("log entry %d: %v", i+1, err)
		}
	}

	entries, err := Recent(database, 3)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
}

func TestRecentForKeepsOneResource(t *testing.T) {
	database := openMigrated(t)

	if err := Log(database, ScopeProject, "proj-1", "Project created", "Etugora"); err != nil {
		t.Fatalf("log first project: %v", err)
	}
	if err := Log(database, ScopeProject, "proj-2", "Project linked", "Other"); err != nil {
		t.Fatalf("log second project: %v", err)
	}
	if err := Log(database, ScopeProject, "proj-1", "Project renamed", "Etugora platform"); err != nil {
		t.Fatalf("log rename: %v", err)
	}
	if err := Log(database, ScopeMachine, "", "Engine started", ""); err != nil {
		t.Fatalf("log machine: %v", err)
	}

	entries, err := RecentFor(database, ScopeProject, "proj-1", 10)
	if err != nil {
		t.Fatalf("recent for: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want only proj-1's two", len(entries))
	}
	if entries[0].Action != "Project renamed" {
		t.Fatalf("newest = %q, want the project's latest action", entries[0].Action)
	}
	for _, entry := range entries {
		if entry.ScopeID != "proj-1" {
			t.Fatalf("entry %q belongs to %q, want proj-1", entry.Action, entry.ScopeID)
		}
	}
}

func TestInvalidScopeRejected(t *testing.T) {
	database := openMigrated(t)

	if err := Log(database, Scope("galaxy"), "", "hitchhike", ""); err == nil {
		t.Fatal("logging with an unknown scope succeeded, want error")
	}
}
