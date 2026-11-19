package runtimes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"locafy.dev/desktop/internal/tools"
)

// Kind is one runtime family an application can pin: the binary the probe
// looks for and the label the interface shows. The registry is data; adding a
// kind needs no new screen.
type Kind struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Bin   string `json:"bin"`
}

// Copy is one installed executable of a kind: a read-only fact of the machine
// that detection never writes, moves or uninstalls.
type Copy struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
	Path    string `json:"path"`
	// Source is how the copy was found: PATH or an install location. A
	// Locafy-managed install arrives with the install work and is a later
	// source value.
	Source string `json:"source"`
}

// Label renders the copy the way the interface names it.
func (c Copy) Label() string {
	version := c.Version
	if version == "" {
		version = "unknown version"
	}
	return fmt.Sprintf("%s %s", filepath.Base(c.Path), version)
}

// kinds are the initial families from PRD section 9.1. Go, Java and Ruby are
// discovered beside PATH the way the other kinds are.
var kinds = []Kind{
	{ID: "php", Label: "PHP", Bin: "php"},
	{ID: "node", Label: "Node.js", Bin: "node"},
	{ID: "python", Label: "Python", Bin: "python"},
	{ID: "go", Label: "Go", Bin: "go"},
	{ID: "java", Label: "Java", Bin: "java"},
	{ID: "ruby", Label: "Ruby", Bin: "ruby"},
	{ID: "dotnet", Label: ".NET", Bin: "dotnet"},
}

// discovery is the seam: tests replace the machine probe with a fixed set.
var discovery = tools.Find

// Catalog reports every kind with the copies found on this machine, newest
// version first. A kind with no copies is listed with none, so the screen can
// say what is missing rather than hiding it.
func Catalog() ([]Kind, [][]Copy, error) {
	list := make([]Kind, len(kinds))
	copiesPerKind := make([][]Copy, len(kinds))
	for index, kind := range kinds {
		list[index] = kind
		copies := make([]Copy, 0)
		for _, candidate := range discovery(kind.Bin) {
			copies = append(copies, Copy{
				ID:      kind.ID + "@" + candidate.Path,
				Kind:    kind.ID,
				Version: candidate.Version,
				Path:    candidate.Path,
				Source:  candidate.Source,
			})
		}
		copiesPerKind[index] = copies
	}
	return list, copiesPerKind, nil
}

// Resolve returns the copy an id names. Identification is by path: the id
// must name a known kind and the file must still exist — the freshness check
// ADR-0018 requires before a start. The version is not re-probed here; the
// catalog carries it.
func Resolve(id string) (*Copy, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil, ErrNoRuntime
	}
	kindID, path, found := strings.Cut(trimmed, "@")
	if !found || path == "" {
		return nil, fmt.Errorf("%w: %q is not a runtime id", ErrUnknownRuntime, id)
	}
	var label string
	known := false
	for _, kind := range kinds {
		if kind.ID == kindID {
			label, known = kind.Label, true
			break
		}
	}
	if !known {
		return nil, fmt.Errorf("%w: %q is not a runtime kind", ErrUnknownRuntime, kindID)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return nil, fmt.Errorf("%w: %s at %s is not installed any more", ErrUnknownRuntime, label, path)
	}
	return &Copy{ID: kindID + "@" + path, Kind: kindID, Path: path}, nil
}

// KindFor maps an application's detected language to a runtime kind, so the
// picker can offer the copies that make sense for it.
func KindFor(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "php":
		return "php"
	case "typescript", "javascript", "node":
		return "node"
	case "python":
		return "python"
	case "go":
		return "go"
	case "java":
		return "java"
	case "ruby":
		return "ruby"
	case "c#", ".net", "csharp":
		return "dotnet"
	default:
		return ""
	}
}

var (
	ErrNoRuntime      = fmt.Errorf("no runtime is assigned")
	ErrUnknownRuntime = fmt.Errorf("unknown runtime")
)

// CatalogEntry pairs a kind with the copies found for it.
type CatalogEntry struct {
	Kind   Kind   `json:"kind"`
	Copies []Copy `json:"copies"`
}

// Service exposes the runtime registry over the application binding layer.
type Service struct{}

// NewService returns a runtimes service. Detection reads the machine and
// stores nothing.
func NewService() *Service {
	return &Service{}
}

// Catalog reports every runtime kind and the copies found on this machine.
func (s *Service) Catalog() ([]CatalogEntry, error) {
	kinds, copiesPerKind, err := Catalog()
	if err != nil {
		return nil, err
	}
	entries := make([]CatalogEntry, len(kinds))
	for index, kind := range kinds {
		entries[index] = CatalogEntry{Kind: kind, Copies: copiesPerKind[index]}
	}
	return entries, nil
}

// KindFor maps an application's detected language to a runtime kind, so the
// picker can offer the copies that make sense for it.
func (s *Service) KindFor(language string) string {
	return KindFor(language)
}
