package settings

import (
	"database/sql"
	"errors"
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

func TestStringRoundTrip(t *testing.T) {
	database := openMigrated(t)

	if err := SetString(database, "default_terminal", "powershell"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := String(database, "default_terminal")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "powershell" {
		t.Fatalf("value = %q, want %q", got, "powershell")
	}

	if err := SetString(database, "default_terminal", "pwsh"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	got, err = String(database, "default_terminal")
	if err != nil {
		t.Fatalf("get after overwrite: %v", err)
	}
	if got != "pwsh" {
		t.Fatalf("value after overwrite = %q, want %q", got, "pwsh")
	}
}

func TestBoolNumberJSONRoundTrips(t *testing.T) {
	database := openMigrated(t)

	if err := SetBool(database, "auto_start_services", true); err != nil {
		t.Fatalf("set bool: %v", err)
	}
	got, err := Bool(database, "auto_start_services")
	if err != nil {
		t.Fatalf("get bool: %v", err)
	}
	if got != true {
		t.Fatalf("bool = %v, want true", got)
	}

	if err := SetNumber(database, "max_projects", 42); err != nil {
		t.Fatalf("set number: %v", err)
	}
	number, err := Number(database, "max_projects")
	if err != nil {
		t.Fatalf("get number: %v", err)
	}
	if number != 42 {
		t.Fatalf("number = %v, want 42", number)
	}

	if err := SetJSON(database, "recent_projects", []string{"etugora", "nestspot"}); err != nil {
		t.Fatalf("set json: %v", err)
	}
	var projects []string
	if err := JSON(database, "recent_projects", &projects); err != nil {
		t.Fatalf("get json: %v", err)
	}
	if len(projects) != 2 || projects[0] != "etugora" {
		t.Fatalf("json = %v, want [etugora nestspot]", projects)
	}
}

func TestTypeMismatchRejected(t *testing.T) {
	database := openMigrated(t)

	if err := SetString(database, "theme", "dark"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := Bool(database, "theme"); err == nil {
		t.Fatal("reading a string setting as bool succeeded, want error")
	}
}

func TestNotFound(t *testing.T) {
	database := openMigrated(t)

	if _, err := Get(database, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if _, err := String(database, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestAllListsEverySetting(t *testing.T) {
	database := openMigrated(t)

	if err := SetString(database, "theme", "dark"); err != nil {
		t.Fatalf("set theme: %v", err)
	}
	if err := SetBool(database, "auto_start_services", false); err != nil {
		t.Fatalf("set auto start: %v", err)
	}

	all, err := All(database)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("settings count = %d, want 2", len(all))
	}
	if all[0].Key != "auto_start_services" || all[1].Key != "theme" {
		t.Fatalf("keys = %q, %q, want ordered by key", all[0].Key, all[1].Key)
	}
}
