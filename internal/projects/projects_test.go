package projects

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"locafy.dev/desktop/internal/db"
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

func activityActions(t *testing.T, database *sql.DB, scopeID string) []string {
	t.Helper()
	rows, err := database.Query(`SELECT action FROM activity_logs WHERE scope_id = ? ORDER BY rowid`, scopeID)
	if err != nil {
		t.Fatalf("query activity: %v", err)
	}
	defer rows.Close()

	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		actions = append(actions, action)
	}
	return actions
}

func TestCreateWritesDirectoryAndRecordOnly(t *testing.T) {
	database := newTestDB(t)
	workspace := t.TempDir()

	project, err := Create(database, workspace, "Etugora")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if project.Kind != KindManaged {
		t.Fatalf("kind = %q, want %q", project.Kind, KindManaged)
	}
	if project.Name != "Etugora" {
		t.Fatalf("name = %q, want %q", project.Name, "Etugora")
	}
	if filepath.Base(project.RootPath) != "etugora" {
		t.Fatalf("root = %q, want the slugged directory name", project.RootPath)
	}

	// Project folders stay clean until an override is requested.
	entries, err := os.ReadDir(project.RootPath)
	if err != nil {
		t.Fatalf("read project directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("project directory holds %d entries, want none", len(entries))
	}

	got, err := Get(database, project.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RootPath != project.RootPath {
		t.Fatalf("stored root = %q, want %q", got.RootPath, project.RootPath)
	}
	if actions := activityActions(t, database, project.ID); len(actions) != 1 || actions[0] != "Project created" {
		t.Fatalf("activity = %v, want a single Project created entry", actions)
	}
}

func TestOverrideFileIsOptInAndOverridesStoredValues(t *testing.T) {
	database := newTestDB(t)

	project, err := Create(database, t.TempDir(), "Etugora")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	path := filepath.Join(project.RootPath, OverrideFile)

	state, err := OverrideStatus(database, project.ID)
	if err != nil {
		t.Fatalf("status before enabling: %v", err)
	}
	if state.Present || state.Path != path {
		t.Fatalf("status = %+v, want absent at %q", state, path)
	}

	if _, err := SetOverrideFile(database, project.ID, true); err != nil {
		t.Fatalf("enable override: %v", err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read override file: %v", err)
	}
	var document struct {
		SchemaVersion int    `json:"schemaVersion"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("decode override file: %v", err)
	}
	if document.SchemaVersion != 1 || document.Name != "Etugora" {
		t.Fatalf("override file = %+v, want schemaVersion 1 and name Etugora", document)
	}

	// Editing the file changes what Locafy shows, without touching the row.
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"name":"Etugora Override"}`), 0o644); err != nil {
		t.Fatalf("edit override file: %v", err)
	}
	got, err := Get(database, project.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Etugora Override" {
		t.Fatalf("name = %q, want the override file's name", got.Name)
	}
	stored, err := projectRow(database, project.ID)
	if err != nil {
		t.Fatalf("stored row: %v", err)
	}
	if stored.Name != "Etugora" {
		t.Fatalf("stored name = %q, want it unchanged at Etugora", stored.Name)
	}

	// A rename while the override is enabled reaches the file, so the
	// operator's own edit is never masked.
	if _, err := Rename(database, project.ID, "Etugora Core"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	renamed, err := Get(database, project.ID)
	if err != nil {
		t.Fatalf("get after rename: %v", err)
	}
	if renamed.Name != "Etugora Core" {
		t.Fatalf("name after rename = %q, want Etugora Core", renamed.Name)
	}

	if _, err := SetOverrideFile(database, project.ID, false); err != nil {
		t.Fatalf("disable override: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat after disabling = %v, want the file removed", err)
	}
	restored, err := Get(database, project.ID)
	if err != nil {
		t.Fatalf("get after disabling: %v", err)
	}
	if restored.Name != "Etugora Core" {
		t.Fatalf("name after disabling = %q, want the stored value", restored.Name)
	}

	actions := activityActions(t, database, project.ID)
	want := []string{"Project created", "Override file written", "Project renamed", "Override file removed"}
	if len(actions) != len(want) {
		t.Fatalf("activity = %v, want %v", actions, want)
	}
	for i := range want {
		if actions[i] != want[i] {
			t.Fatalf("activity = %v, want %v", actions, want)
		}
	}
}

func TestBrokenOverrideFileLeavesStoredValues(t *testing.T) {
	database := newTestDB(t)

	project, err := Create(database, t.TempDir(), "Etugora")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	path := filepath.Join(project.RootPath, OverrideFile)
	for _, payload := range []string{"{not json", `{"schemaVersion":2,"name":"Later"}`, `{"schemaVersion":1,"name":"   "}`} {
		if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
			t.Fatalf("write override file: %v", err)
		}
		got, err := Get(database, project.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Name != "Etugora" {
			t.Fatalf("name with %s = %q, want the stored name", payload, got.Name)
		}
	}
}

func TestCreateSlugCollisionsGetNumericSuffix(t *testing.T) {
	database := newTestDB(t)
	workspace := t.TempDir()

	first, err := Create(database, workspace, "Etugora")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second, err := Create(database, workspace, "Etugora")
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if second.RootPath == first.RootPath {
		t.Fatalf("both projects share %q, want distinct directories", first.RootPath)
	}
	if want := first.RootPath + "-2"; second.RootPath != want {
		t.Fatalf("second root = %q, want %q", second.RootPath, want)
	}
}

func TestCreateRejectsBlankAndOverlongNames(t *testing.T) {
	database := newTestDB(t)
	workspace := t.TempDir()

	if _, err := Create(database, workspace, "   "); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("blank name error = %v, want ErrNameRequired", err)
	}
	if _, err := Create(database, workspace, strings.Repeat("a", 121)); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("overlong name error = %v, want ErrNameRequired", err)
	}
}

func TestCreateRemovesDirectoryWhenInsertFails(t *testing.T) {
	database := newTestDB(t)
	workspace := t.TempDir()

	// A live row already owns the directory this create will allocate.
	reserved := filepath.Join(workspace, "ghost")
	if _, err := database.Exec(`INSERT INTO projects (id, name, root_path, kind) VALUES ('ghost', 'Ghost', ?, 'linked')`, reserved); err != nil {
		t.Fatalf("seed row: %v", err)
	}

	if _, err := Create(database, workspace, "Ghost"); !errors.Is(err, ErrPathInUse) {
		t.Fatalf("create error = %v, want ErrPathInUse", err)
	}
	if _, err := os.Stat(reserved); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat after failed create = %v, want the directory removed", err)
	}
}

func TestLinkRegistersExistingDirectoryWithoutWriting(t *testing.T) {
	database := newTestDB(t)
	existing := t.TempDir()

	project, err := Link(database, existing)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if project.Kind != KindLinked {
		t.Fatalf("kind = %q, want %q", project.Kind, KindLinked)
	}
	if project.RootPath != existing {
		t.Fatalf("root = %q, want %q", project.RootPath, existing)
	}
	entries, err := os.ReadDir(existing)
	if err != nil {
		t.Fatalf("read linked directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("linked directory holds %d entries, want nothing written into it", len(entries))
	}
	if actions := activityActions(t, database, project.ID); len(actions) != 1 || actions[0] != "Project linked" {
		t.Fatalf("activity = %v, want a single Project linked entry", actions)
	}
}

func TestLinkRejectsMissingFileAndBlankPaths(t *testing.T) {
	database := newTestDB(t)

	if _, err := Link(database, filepath.Join(t.TempDir(), "absent")); !errors.Is(err, ErrPathMissing) {
		t.Fatalf("missing path error = %v, want ErrPathMissing", err)
	}

	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := Link(database, file); !errors.Is(err, ErrPathNotDir) {
		t.Fatalf("file path error = %v, want ErrPathNotDir", err)
	}

	if _, err := Link(database, "  "); !errors.Is(err, ErrPathRequired) {
		t.Fatalf("blank path error = %v, want ErrPathRequired", err)
	}
}

func TestLinkRejectsDirectoryUsedByLiveProjectIgnoringCase(t *testing.T) {
	database := newTestDB(t)
	existing := t.TempDir()

	// Seed a row whose stored path differs only by case; the uniqueness
	// check compares case-insensitively on every platform.
	if _, err := database.Exec(`INSERT INTO projects (id, name, root_path, kind) VALUES ('other', 'Other', ?, 'linked')`, strings.ToUpper(existing)); err != nil {
		t.Fatalf("seed row: %v", err)
	}

	if _, err := Link(database, existing); !errors.Is(err, ErrPathInUse) {
		t.Fatalf("link error = %v, want ErrPathInUse", err)
	}
}

func TestRenameUpdatesNameAndLogsActivity(t *testing.T) {
	database := newTestDB(t)

	project, err := Link(database, t.TempDir())
	if err != nil {
		t.Fatalf("link: %v", err)
	}

	renamed, err := Rename(database, project.ID, "Etugora Core")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Name != "Etugora Core" {
		t.Fatalf("name = %q, want %q", renamed.Name, "Etugora Core")
	}
	if _, err := Rename(database, project.ID, "   "); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("blank rename error = %v, want ErrNameRequired", err)
	}
	if _, err := Rename(database, "missing", "Name"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id error = %v, want ErrNotFound", err)
	}
	if actions := activityActions(t, database, project.ID); len(actions) != 2 || actions[1] != "Project renamed" {
		t.Fatalf("activity = %v, want Project linked then Project renamed", actions)
	}
}

func TestChangeRootRejectsManagedProjects(t *testing.T) {
	database := newTestDB(t)

	project, err := Create(database, t.TempDir(), "Managed")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	original := project.RootPath

	if _, err := ChangeRoot(database, project.ID, t.TempDir()); !errors.Is(err, ErrManagedRootFixed) {
		t.Fatalf("change root error = %v, want ErrManagedRootFixed", err)
	}
	got, err := Get(database, project.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RootPath != original {
		t.Fatalf("root = %q, want it unchanged at %q", got.RootPath, original)
	}
}

func TestChangeRootValidatesTargetAndNeverMovesFiles(t *testing.T) {
	database := newTestDB(t)
	oldDir := t.TempDir()
	project, err := Link(database, oldDir)
	if err != nil {
		t.Fatalf("link: %v", err)
	}

	if _, err := ChangeRoot(database, project.ID, filepath.Join(t.TempDir(), "absent")); !errors.Is(err, ErrPathMissing) {
		t.Fatalf("missing target error = %v, want ErrPathMissing", err)
	}

	newDir := t.TempDir()
	if _, err := Link(database, newDir); err != nil {
		t.Fatalf("link second: %v", err)
	}
	if _, err := ChangeRoot(database, project.ID, newDir); !errors.Is(err, ErrPathInUse) {
		t.Fatalf("taken target error = %v, want ErrPathInUse", err)
	}

	fresh := t.TempDir()
	changed, err := ChangeRoot(database, project.ID, fresh)
	if err != nil {
		t.Fatalf("change root: %v", err)
	}
	if changed.RootPath != fresh {
		t.Fatalf("root = %q, want %q", changed.RootPath, fresh)
	}
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		t.Fatalf("read old directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("old directory holds %d entries, want it untouched", len(entries))
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("stat new directory: %v", err)
	}
	if _, err := ChangeRoot(database, "missing", fresh); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id error = %v, want ErrNotFound", err)
	}
}

func TestRemoveKeepsFilesAndReleasesThePath(t *testing.T) {
	database := newTestDB(t)
	workspace := t.TempDir()

	project, err := Create(database, workspace, "Keeper")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := Remove(database, project.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, err := Get(database, project.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after remove error = %v, want ErrNotFound", err)
	}
	live, err := List(database)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(live) != 0 {
		t.Fatalf("list holds %d projects after remove, want none", len(live))
	}

	// The directory stays on disk: removal never deletes source files.
	if _, err := os.Stat(project.RootPath); err != nil {
		t.Fatalf("stat project directory: %v", err)
	}

	// The row released its path: another project may now use it.
	if _, err := Link(database, project.RootPath); err != nil {
		t.Fatalf("reuse released path: %v", err)
	}
	if err := Remove(database, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("remove unknown error = %v, want ErrNotFound", err)
	}
	if actions := activityActions(t, database, project.ID); len(actions) != 2 || actions[1] != "Project removed" {
		t.Fatalf("activity = %v, want Project created then Project removed", actions)
	}
}

func TestListOrdersLiveProjectsByName(t *testing.T) {
	database := newTestDB(t)
	workspace := t.TempDir()

	for _, name := range []string{"beta", "Alpha"} {
		if _, err := Create(database, workspace, name); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}
	removed, err := Create(database, workspace, "Gone")
	if err != nil {
		t.Fatalf("create removed: %v", err)
	}
	if err := Remove(database, removed.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	live, err := List(database)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(live) != 2 {
		t.Fatalf("list holds %d projects, want 2", len(live))
	}
	if live[0].Name != "Alpha" || live[1].Name != "beta" {
		t.Fatalf("order = [%q %q], want case-insensitive name order", live[0].Name, live[1].Name)
	}
}

func TestServiceWorkspacePreferenceOverridesTheDefault(t *testing.T) {
	database := newTestDB(t)
	service := NewService(database)

	def, err := service.workspaceDir()
	if err != nil {
		t.Fatalf("default workspace: %v", err)
	}
	wantDefault, err := db.DefaultProjectsDir()
	if err != nil {
		t.Fatalf("platform default: %v", err)
	}
	if def != wantDefault {
		t.Fatalf("workspace = %q without a preference, want %q", def, wantDefault)
	}

	chosen := t.TempDir()
	if err := settings.SetString(database, WorkspaceRootKey, chosen); err != nil {
		t.Fatalf("store preference: %v", err)
	}
	got, err := service.workspaceDir()
	if err != nil {
		t.Fatalf("preferred workspace: %v", err)
	}
	if got != chosen {
		t.Fatalf("workspace = %q with a preference, want %q", got, chosen)
	}

	if err := settings.SetString(database, WorkspaceRootKey, "  "); err != nil {
		t.Fatalf("store blank preference: %v", err)
	}
	got, err = service.workspaceDir()
	if err != nil {
		t.Fatalf("blank preference workspace: %v", err)
	}
	if got != wantDefault {
		t.Fatalf("workspace = %q for a blank preference, want %q", got, wantDefault)
	}

	if err := settings.SetString(database, WorkspaceRootKey, "relative/path"); err != nil {
		t.Fatalf("store relative preference: %v", err)
	}
	if _, err := service.workspaceDir(); !errors.Is(err, ErrWorkspaceInvalid) {
		t.Fatalf("relative preference error = %v, want ErrWorkspaceInvalid", err)
	}
}

func TestServiceCreateUsesThePreferredWorkspace(t *testing.T) {
	database := newTestDB(t)
	chosen := t.TempDir()
	if err := settings.SetString(database, WorkspaceRootKey, chosen); err != nil {
		t.Fatalf("store preference: %v", err)
	}

	project, err := NewService(database).Create("Preferred")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(project.RootPath, chosen) {
		t.Fatalf("root = %q, want it under the preferred workspace %q", project.RootPath, chosen)
	}
}

func TestServiceOpensTheProjectFolder(t *testing.T) {
	database := newTestDB(t)
	service := NewService(database)
	project, err := service.Create("Etugora")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	opened := ""
	restore := openProjectFolder
	openProjectFolder = func(path string) error {
		opened = path
		return nil
	}
	t.Cleanup(func() { openProjectFolder = restore })

	if err := service.OpenFolder(project.ID); err != nil {
		t.Fatalf("OpenFolder: %v", err)
	}
	if opened != project.RootPath {
		t.Fatalf("opened %q, want %q", opened, project.RootPath)
	}
	if err := service.OpenFolder("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown project error = %v, want ErrNotFound", err)
	}
}

func TestServiceActivityKeepsOneProject(t *testing.T) {
	database := newTestDB(t)
	service := NewService(database)
	project, err := service.Create("Etugora")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	other, err := service.Create("Other")
	if err != nil {
		t.Fatalf("create other: %v", err)
	}
	if _, err := service.Rename(project.ID, "Etugora platform"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	entries, err := service.Activity(project.ID, 10)
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want the project's two actions", len(entries))
	}
	if entries[0].Action != "Project renamed" {
		t.Fatalf("newest = %q, want Project renamed", entries[0].Action)
	}
	for _, entry := range entries {
		if entry.ScopeID != project.ID {
			t.Fatalf("entry belongs to %q, want %q", entry.ScopeID, project.ID)
		}
		if entry.ScopeID == other.ID {
			t.Fatal("another project's activity leaked into the feed")
		}
	}
	if _, err := service.Activity("missing", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown project error = %v, want ErrNotFound", err)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(errors.New("UNIQUE constraint failed: projects.root_path")) {
		t.Fatal("unique constraint message not recognised")
	}
	if isUniqueViolation(errors.New("CHECK constraint failed")) {
		t.Fatal("unrelated error recognised as a unique violation")
	}
	if isUniqueViolation(nil) {
		t.Fatal("nil recognised as a unique violation")
	}
}
