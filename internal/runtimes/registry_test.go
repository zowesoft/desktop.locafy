package runtimes

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"locafy.dev/desktop/internal/tools"
)

func withDiscovery(t *testing.T, candidates func(name string) []tools.Candidate) {
	t.Helper()
	restore := discovery
	t.Cleanup(func() { discovery = restore })
	discovery = candidates
}

func TestCatalogListsEveryKindAndItsCopies(t *testing.T) {
	withDiscovery(t, func(name string) []tools.Candidate {
		if name == "php" {
			return []tools.Candidate{
				{Path: `C:\laragon\bin\php\php-8.3\php.exe`, Version: "8.3.7", Source: "installed"},
				{Path: `C:\tools\php\php.exe`, Version: "8.2.12", Source: "PATH"},
			}
		}
		return nil
	})

	kinds, copies, err := Catalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	entries := make([]CatalogEntry, len(kinds))
	for index, kind := range kinds {
		entries[index] = CatalogEntry{Kind: kind, Copies: copies[index]}
	}
	if len(kinds) != 7 {
		t.Fatalf("kinds = %d, want 7", len(kinds))
	}
	// The first entry is the newest, and a kind without copies is still
	// listed, so the screen can say "not installed" instead of hiding it.
	var found bool
	for _, entry := range entries {
		if entry.Kind.ID != "php" {
			if len(entry.Copies) != 0 {
				t.Errorf("kind %q carries copies %v", entry.Kind.ID, entry.Copies)
			}
			continue
		}
		found = true
		if len(entry.Copies) != 2 {
			t.Fatalf("php copies = %d, want 2", len(entry.Copies))
		}
		if entry.Copies[0].Version != "8.3.7" {
			t.Errorf("first copy = %q, want the newest", entry.Copies[0].Version)
		}
		if entry.Copies[0].Label() == "" {
			t.Error("copy label is empty")
		}
	}
	if !found {
		t.Fatal("php is missing from the catalog")
	}
}

func TestResolveReturnsTheCopyAPathNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "python.exe")
	if err := os.WriteFile(path, []byte("#stub"), 0o755); err != nil {
		t.Fatal(err)
	}

	copy, err := Resolve("python@" + path)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if copy.Kind != "python" || copy.Path != path {
		t.Errorf("copy = %+v, want the named path", copy)
	}
}

func TestResolveRefusesWhatIsNotThere(t *testing.T) {
	if _, err := Resolve(""); !errors.Is(err, ErrNoRuntime) {
		t.Errorf("empty id err = %v, want the no-runtime refusal", err)
	}
	if _, err := Resolve("python"); !errors.Is(err, ErrUnknownRuntime) {
		t.Errorf("bare id err = %v, want the unknown-runtime refusal", err)
	}
	if _, err := Resolve("fortran@C:\\nowhere\\f90.exe"); !errors.Is(err, ErrUnknownRuntime) {
		t.Errorf("unknown kind err = %v, want the unknown-runtime refusal", err)
	}
	if _, err := Resolve("python@C:\\nowhere\\python.exe"); err == nil || !strings.Contains(err.Error(), "not installed any more") {
		t.Errorf("gone path err = %v, want the gone refusal", err)
	}
}

func TestKindForMapsALanguageToItsRuntime(t *testing.T) {
	if got := KindFor("TypeScript"); got != "node" {
		t.Errorf("KindFor(TypeScript) = %q, want node", got)
	}
	if got := KindFor("PHP"); got != "php" {
		t.Errorf("KindFor(PHP) = %q, want php", got)
	}
}
