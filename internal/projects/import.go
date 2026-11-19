package projects

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FolderCandidate is one child directory offered by a scan. Marker names the
// first recognizable project marker found in it; it is advisory only and is
// empty when nothing was recognized.
type FolderCandidate struct {
	Path   string
	Name   string
	Marker string
}

// ImportOutcome reports one directory's result in a bulk import. Error is
// empty when the directory is now a linked project.
type ImportOutcome struct {
	Path  string
	Error string
}

// ScanResult is what one scan offers: the selected folder itself, with the
// marker its own contents suggest, and the child directories inside it that
// are not already projects. Root lets the caller present importing the folder
// as a single project alongside importing its children separately.
type ScanResult struct {
	Root     FolderCandidate
	Children []FolderCandidate
}

// ScanFolder lists the child directories of rootPath that are not already
// project roots, marking the ones that look like projects, together with the
// selected folder itself. It reads directory entries and marker file names
// only; nothing is written or executed.
func ScanFolder(database *sql.DB, rootPath string) (ScanResult, error) {
	root, err := resolveRoot(rootPath)
	if err != nil {
		return ScanResult{}, err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return ScanResult{}, fmt.Errorf("read %q: %w", root, err)
	}
	linked, err := liveRoots(database)
	if err != nil {
		return ScanResult{}, err
	}

	result := ScanResult{
		Root: FolderCandidate{
			Path:   root,
			Name:   filepath.Base(root),
			Marker: detectMarker(root),
		},
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if linked[strings.ToLower(path)] {
			continue
		}
		result.Children = append(result.Children, FolderCandidate{
			Path:   path,
			Name:   entry.Name(),
			Marker: detectMarker(path),
		})
	}
	return result, nil
}

// LinkMany links each selected directory as a linked project. Every path is
// attempted so one failure does not stop the rest, and each result is
// reported back to the caller.
func LinkMany(database *sql.DB, paths []string) ([]ImportOutcome, error) {
	if len(paths) == 0 {
		return nil, ErrPathRequired
	}
	outcomes := make([]ImportOutcome, 0, len(paths))
	for _, path := range paths {
		if _, err := Link(database, path); err != nil {
			outcomes = append(outcomes, ImportOutcome{Path: path, Error: err.Error()})
			continue
		}
		outcomes = append(outcomes, ImportOutcome{Path: path})
	}
	return outcomes, nil
}

// liveRoots maps every live project root to lower case, mirroring the
// case-insensitive path comparison the database performs.
func liveRoots(database *sql.DB) (map[string]bool, error) {
	rows, err := database.Query(`SELECT root_path FROM projects WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("list project roots: %w", err)
	}
	defer rows.Close()

	roots := make(map[string]bool)
	for rows.Next() {
		var root string
		if err := rows.Scan(&root); err != nil {
			return nil, fmt.Errorf("scan project root: %w", err)
		}
		roots[strings.ToLower(root)] = true
	}
	return roots, rows.Err()
}

// detectMarker reports the first recognizable project marker in dir. A marker
// suggests a project but never proves one, so the result stays advisory.
func detectMarker(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if marker := matchMarker(entry.Name()); marker != "" {
			return marker
		}
	}
	return ""
}

// matchMarker recognizes the marker files that suggest a project, covering
// the exact names plus the wildcard forms.
func matchMarker(name string) string {
	switch name {
	case OverrideFile, "composer.json", "artisan", "package.json", "nest-cli.json",
		"pyproject.toml", "requirements.txt", "manage.py", "go.mod", "Gemfile":
		return name
	}
	if strings.HasPrefix(name, "next.config.") || strings.HasSuffix(name, ".csproj") || strings.HasSuffix(name, ".sln") {
		return name
	}
	return ""
}
