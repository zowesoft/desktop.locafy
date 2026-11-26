package logs

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"locafy.dev/desktop/internal/db"
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

func TestMaskRemovesCredentials(t *testing.T) {
	cases := []struct {
		line   string
		want   string
		masked bool
	}{
		{"connected as user password=hunter2 on port 5432", "connected as user password=••• on port 5432", true},
		{"DB_PASSWORD: s3cr3t-value", "DB_PASSWORD: •••", true},
		{"Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.abc", "Authorization: •••", true},
		{"calling https://user:pw@example.test/api", "calling https://user:•••@example.test/api", true},
		{"api_key=abc123&page=2", "api_key=•••&page=2", true},
		{"listening on 127.0.0.1:3000", "listening on 127.0.0.1:3000", false},
		{"token count 42 tokens", "token count 42 tokens", false},
	}
	for _, testCase := range cases {
		got, masked := Mask(testCase.line)
		if got != testCase.want || masked != testCase.masked {
			t.Fatalf("Mask(%q) = %q, %v; want %q, %v", testCase.line, got, masked, testCase.want, testCase.masked)
		}
	}
}

func TestStoreKeepsTheNewestLinesWithinItsCapacity(t *testing.T) {
	store := NewStore(nil, 3)

	for _, line := range []string{"one", "two", "three", "four", "five"} {
		store.Record(LayerApplication, "app-1", line)
	}

	entries := store.Query(Filter{})
	if len(entries) != 3 {
		t.Fatalf("query returned %d entries, want the capacity of 3", len(entries))
	}
	for index, want := range []string{"three", "four", "five"} {
		if entries[index].Line != want {
			t.Fatalf("entry %d = %q, want %q (oldest first)", index, entries[index].Line, want)
		}
	}
}

func TestStoreFiltersByLayerSourceAndText(t *testing.T) {
	store := NewStore(nil, 0)
	store.Record(LayerApplication, "app-1", "vite ready in 240ms")
	store.Record(LayerService, "svc-1", "database system is ready to accept connections")
	store.Record(LayerApplication, "app-2", "GET /health 200")

	byLayer := store.Query(Filter{Layer: LayerService})
	if len(byLayer) != 1 || byLayer[0].Line == "" || !strings.Contains(byLayer[0].Line, "database") {
		t.Fatalf("layer filter = %+v, want the one service line", byLayer)
	}
	bySource := store.Query(Filter{SourceID: "app-1"})
	if len(bySource) != 1 || !strings.Contains(bySource[0].Line, "vite") {
		t.Fatalf("source filter = %+v, want the one app-1 line", bySource)
	}
	byText := store.Query(Filter{Contains: "HEALTH"})
	if len(byText) != 1 || byText[0].SourceID != "app-2" {
		t.Fatalf("text filter = %+v, want the health request", byText)
	}
	if limited := store.Query(Filter{Limit: 2}); len(limited) != 2 || limited[1].SourceID != "app-2" {
		t.Fatalf("limit = %+v, want the newest two lines", limited)
	}
}

func TestStoreScopesEntriesToTheirProject(t *testing.T) {
	database := newTestDB(t)
	store := NewStore(database, 0)

	insertProject := func(id, name, root string) {
		if _, err := database.Exec(`INSERT INTO projects (id, name, root_path, kind) VALUES (?, ?, ?, 'linked')`, id, name, root); err != nil {
			t.Fatalf("insert project %q: %v", name, err)
		}
	}
	insertProject("p1", "Etugora", "C:/work/etugora")
	insertProject("p2", "Portfolio", "C:/work/portfolio")

	if _, err := database.Exec(`INSERT INTO applications (id, project_id, name, directory, start_command)
		VALUES ('app-1', 'p1', 'api', '.', 'php artisan serve')`); err != nil {
		t.Fatalf("insert application: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO services (id, project_id, name, engine, port, data_directory)
		VALUES ('svc-1', 'p2', 'PostgreSQL', 'postgresql', 5433, 'C:/locafy/services/svc-1')`); err != nil {
		t.Fatalf("insert service: %v", err)
	}

	store.Record(LayerApplication, "app-1", "api booted")
	store.Record(LayerService, "svc-1", "postgres booted")

	scoped := store.Query(Filter{ProjectID: "p1"})
	if len(scoped) != 1 || scoped[0].Line != "api booted" || scoped[0].Source != "api" {
		t.Fatalf("p1 entries = %+v, want only api's line named api", scoped)
	}
	if other := store.Query(Filter{ProjectID: "p2"}); len(other) != 1 || other[0].Source != "PostgreSQL" {
		t.Fatalf("p2 entries = %+v, want only the service's line named PostgreSQL", other)
	}
	if all := store.Query(Filter{}); len(all) != 2 {
		t.Fatalf("unscoped query returned %d entries, want both", len(all))
	}
}
