package projects

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"locafy.dev/desktop/internal/activity"
)

// OverrideState describes a project's optional override file: where it would
// live and whether the operator has enabled it.
type OverrideState struct {
	Path    string
	Present bool
}

// overrideDocument is the on-disk format of the override file. Every field is
// a pointer so an absent key keeps the stored value instead of clearing it.
type overrideDocument struct {
	SchemaVersion int     `json:"schemaVersion"`
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
}

// OverrideStatus reports where a project's override file lives and whether it
// is present. Nothing is created.
func OverrideStatus(database *sql.DB, id string) (OverrideState, error) {
	project, err := projectRow(database, id)
	if err != nil {
		return OverrideState{}, err
	}
	path := filepath.Join(project.RootPath, OverrideFile)
	if _, err := os.Stat(path); err == nil {
		return OverrideState{Path: path, Present: true}, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return OverrideState{}, fmt.Errorf("inspect override file: %w", err)
	}
	return OverrideState{Path: path}, nil
}

// SetOverrideFile writes or removes a project's override file. Writing uses
// the stored values, so enabling it alone changes nothing until the file is
// edited; this is the only path that creates the file.
func SetOverrideFile(database *sql.DB, id string, enabled bool) (OverrideState, error) {
	project, err := projectRow(database, id)
	if err != nil {
		return OverrideState{}, err
	}
	path := filepath.Join(project.RootPath, OverrideFile)

	if !enabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return OverrideState{Path: path}, fmt.Errorf("remove override file: %w", err)
		}
		if err := activity.Log(database, activity.ScopeProject, id, "Override file removed", project.Name); err != nil {
			return OverrideState{Path: path}, err
		}
		return OverrideState{Path: path}, nil
	}

	if err := writeOverride(path, project.Name, project.Description); err != nil {
		return OverrideState{Path: path}, err
	}
	state := OverrideState{Path: path, Present: true}
	if err := activity.Log(database, activity.ScopeProject, id, "Override file written", project.Name); err != nil {
		return state, err
	}
	return state, nil
}

func writeOverride(path string, name string, description string) error {
	document := overrideDocument{SchemaVersion: 1, Name: &name, Description: &description}
	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode override file: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write override file: %w", err)
	}
	return nil
}

// applyOverride replaces a project's stored name and description with the
// values in its override file. A missing, unreadable or unrecognised file
// leaves the record untouched, and an empty name is ignored.
func applyOverride(project Project) Project {
	payload, err := os.ReadFile(filepath.Join(project.RootPath, OverrideFile))
	if err != nil {
		return project
	}
	var document overrideDocument
	if err := json.Unmarshal(payload, &document); err != nil || document.SchemaVersion != 1 {
		return project
	}
	if document.Name != nil {
		if name := strings.TrimSpace(*document.Name); name != "" && len([]rune(name)) <= 120 {
			project.Name = name
		}
	}
	if document.Description != nil {
		project.Description = strings.TrimSpace(*document.Description)
	}
	return project
}

// syncOverride rewrites an existing override file after a stored change, so a
// file the operator opted into never masks their edit.
func syncOverride(project Project) error {
	path := filepath.Join(project.RootPath, OverrideFile)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect override file: %w", err)
	}
	return writeOverride(path, project.Name, project.Description)
}
