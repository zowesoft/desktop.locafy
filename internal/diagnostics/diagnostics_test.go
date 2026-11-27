package diagnostics

import (
	"archive/zip"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/logs"
	"locafy.dev/desktop/internal/settings"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func fileNamed(t *testing.T, bundle Bundle, name string) File {
	t.Helper()
	for _, file := range bundle.Files {
		if file.Name == name {
			return file
		}
	}
	t.Fatalf("bundle has no %s; has %v", name, names(bundle))
	return File{}
}

func names(bundle Bundle) []string {
	found := make([]string, 0, len(bundle.Files))
	for _, file := range bundle.Files {
		found = append(found, file.Name)
	}
	return found
}

func TestBuildDescribesTheEngineAndMasksSecrets(t *testing.T) {
	database := newTestDB(t)
	if err := settings.SetString(database, "database.url", "postgres://app:hunter2@localhost/db"); err != nil {
		t.Fatalf("store setting: %v", err)
	}
	if err := settings.SetString(database, "ui.theme", "dark"); err != nil {
		t.Fatalf("store setting: %v", err)
	}

	store := logs.NewStore(database, 10)
	store.Record(logs.LayerApplication, "app-1", "connecting with password=hunter2 to the database")

	bundle, err := Build(Input{
		Database: database,
		LogLines: store.Query(logs.Filter{}),
		DataDir:  "/data/locafy",
		LogPath:  "/data/locafy/locafy.log",
		StartedAt: time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC),
		Now:       time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	system := fileNamed(t, bundle, "system.txt")
	if !strings.Contains(system.Content, "platform: ") || !strings.Contains(system.Content, "locafy.db") {
		t.Fatalf("system summary = %q, want platform and database path", system.Content)
	}

	settingsFile := fileNamed(t, bundle, "settings.txt")
	if strings.Contains(settingsFile.Content, "hunter2") {
		t.Fatalf("settings still contain the password: %q", settingsFile.Content)
	}
	if !settingsFile.Masked {
		t.Fatal("settings file is not marked as masked")
	}
	if !strings.Contains(settingsFile.Content, "ui.theme [string] = dark") {
		t.Fatalf("settings file lost an ordinary value: %q", settingsFile.Content)
	}

	logFile := fileNamed(t, bundle, "logs.txt")
	if strings.Contains(logFile.Content, "hunter2") {
		t.Fatalf("log file still contains the password: %q", logFile.Content)
	}
	if !strings.Contains(logFile.Content, "Engine started") && !strings.Contains(logFile.Content, "connecting with") {
		t.Fatalf("log file lost the captured line: %q", logFile.Content)
	}

	counts := fileNamed(t, bundle, "counts.txt")
	if !strings.Contains(counts.Content, "schema migrations applied: ") {
		t.Fatalf("counts = %q, want the migration count", counts.Content)
	}
}

func TestBuildReportsAnEmptyEngineInsteadOfFailing(t *testing.T) {
	bundle, err := Build(Input{Now: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("build without a database: %v", err)
	}
	counts := fileNamed(t, bundle, "counts.txt")
	if !strings.Contains(counts.Content, "(no database)") {
		t.Fatalf("counts = %q, want a no-database note", counts.Content)
	}
	if logged := fileNamed(t, bundle, "logs.txt"); !strings.Contains(logged.Content, "(no output captured)") {
		t.Fatalf("logs = %q, want the empty note", logged.Content)
	}
	if len(bundle.Files) != 5 {
		t.Fatalf("bundle has %v, want five members", names(bundle))
	}
}

func TestWriteArchiveRoundTripsEveryMember(t *testing.T) {
	bundle := Bundle{GeneratedAt: "2026-11-27T10:00:00Z", Files: []File{
		{Name: "system.txt", Content: "platform: windows/amd64\n"},
		{Name: "logs.txt", Content: "line one\nline two\n"},
	}}
	path := filepath.Join(t.TempDir(), "bundle.zip")
	if err := WriteArchive(bundle, path); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer reader.Close()

	got := map[string]string{}
	for _, member := range reader.File {
		file, err := member.Open()
		if err != nil {
			t.Fatalf("open %s: %v", member.Name, err)
		}
		content, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			t.Fatalf("read %s: %v", member.Name, err)
		}
		got[member.Name] = string(content)
	}
	if got["system.txt"] != "platform: windows/amd64\n" || got["logs.txt"] != "line one\nline two\n" {
		t.Fatalf("archive contents = %v", got)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("archive on disk: %v, %v", info, err)
	}
}

func TestDefaultFilenameCarriesTheMoment(t *testing.T) {
	name := DefaultFilename(time.Date(2026, 11, 27, 9, 5, 7, 0, time.UTC))
	if !strings.HasPrefix(name, "locafy-diagnostics-") || !strings.HasSuffix(name, ".zip") {
		t.Fatalf("filename = %q, want a dated diagnostics archive", name)
	}
}
