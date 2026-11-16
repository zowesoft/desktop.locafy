package projects

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScanFolderListsChildrenWithAdvisoryMarkers(t *testing.T) {
	database := newTestDB(t)
	root := t.TempDir()

	withMarker := filepath.Join(root, "etugora")
	if err := os.Mkdir(withMarker, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(withMarker, "composer.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	suffixMarker := filepath.Join(root, "webapp")
	if err := os.Mkdir(suffixMarker, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(suffixMarker, "site.csproj"), []byte(""), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	plain := filepath.Join(root, "uploads")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, ".hidden"), 0o755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	linked := filepath.Join(root, "already")
	if err := os.Mkdir(linked, 0o755); err != nil {
		t.Fatalf("mkdir linked: %v", err)
	}
	if _, err := Link(database, linked); err != nil {
		t.Fatalf("link: %v", err)
	}

	got, err := ScanFolder(database, root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	markers := make(map[string]string, len(got))
	for _, candidate := range got {
		markers[candidate.Name] = candidate.Marker
		if want := filepath.Join(root, candidate.Name); candidate.Path != want {
			t.Errorf("path = %q, want %q", candidate.Path, want)
		}
	}
	if len(got) != 3 {
		t.Fatalf("candidates = %d (%+v), want the three unlinked directories", len(got), got)
	}
	if markers["etugora"] != "composer.json" {
		t.Errorf("etugora marker = %q, want composer.json", markers["etugora"])
	}
	if markers["webapp"] != "site.csproj" {
		t.Errorf("webapp marker = %q, want site.csproj", markers["webapp"])
	}
	if markers["uploads"] != "" {
		t.Errorf("uploads marker = %q, want it unrecognized", markers["uploads"])
	}
	if _, ok := markers[".hidden"]; ok {
		t.Error("hidden directory offered as a candidate")
	}
	if _, ok := markers["already"]; ok {
		t.Error("already linked directory offered as a candidate")
	}
}

func TestScanFolderValidatesTheSelectedPath(t *testing.T) {
	database := newTestDB(t)

	if _, err := ScanFolder(database, "  "); !errors.Is(err, ErrPathRequired) {
		t.Fatalf("blank path error = %v, want ErrPathRequired", err)
	}
	if _, err := ScanFolder(database, filepath.Join(t.TempDir(), "absent")); !errors.Is(err, ErrPathMissing) {
		t.Fatalf("missing path error = %v, want ErrPathMissing", err)
	}

	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := ScanFolder(database, file); !errors.Is(err, ErrPathNotDir) {
		t.Fatalf("file path error = %v, want ErrPathNotDir", err)
	}
}

func TestLinkManyImportsEveryPathAndReportsFailures(t *testing.T) {
	database := newTestDB(t)
	root := t.TempDir()

	first := filepath.Join(root, "alpha")
	second := filepath.Join(root, "beta")
	for _, dir := range []string{first, second} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	missing := filepath.Join(root, "absent")

	outcomes, err := LinkMany(database, []string{first, missing, second})
	if err != nil {
		t.Fatalf("link many: %v", err)
	}
	if len(outcomes) != 3 {
		t.Fatalf("outcomes = %d, want one per path", len(outcomes))
	}
	if outcomes[0].Path != first || outcomes[0].Error != "" {
		t.Errorf("first outcome = %+v, want it linked without error", outcomes[0])
	}
	if outcomes[1].Path != missing || outcomes[1].Error == "" {
		t.Errorf("missing outcome = %+v, want the failure reported", outcomes[1])
	}
	if outcomes[2].Path != second || outcomes[2].Error != "" {
		t.Errorf("second outcome = %+v, want it linked despite the earlier failure", outcomes[2])
	}

	live, err := List(database)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(live) != 2 {
		t.Fatalf("projects = %d, want the two existing directories linked", len(live))
	}

	if _, err := LinkMany(database, nil); !errors.Is(err, ErrPathRequired) {
		t.Fatalf("empty selection error = %v, want ErrPathRequired", err)
	}
}
