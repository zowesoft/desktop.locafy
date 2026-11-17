package applications

import (
	"database/sql"
	"errors"
	"fmt"

	"locafy.dev/desktop/internal/db"
)

var (
	ErrSelfDependency           = errors.New("an application cannot depend on itself")
	ErrDependencyOutsideProject = errors.New("a dependency must be another application in the same project")
	ErrDependencyCycle          = errors.New("applications cannot depend on each other in a cycle")
)

// SetDependencies replaces what an application depends on. The whole graph is
// validated before anything is written, so a cycle is reported instead of
// stored and the previous edges survive the refusal.
func SetDependencies(database *sql.DB, applicationID string, dependsOn []string) error {
	app, err := Get(database, applicationID)
	if err != nil {
		return err
	}

	apps, err := List(database, app.ProjectID)
	if err != nil {
		return err
	}
	inProject := make(map[string]bool, len(apps))
	edges := make(map[string][]string, len(apps))
	for _, candidate := range apps {
		inProject[candidate.ID] = true
		edges[candidate.ID] = append([]string(nil), candidate.DependsOn...)
	}

	wanted := make([]string, 0, len(dependsOn))
	seen := make(map[string]bool, len(dependsOn))
	for _, dependency := range dependsOn {
		if dependency == app.ID {
			return ErrSelfDependency
		}
		if !inProject[dependency] {
			return ErrDependencyOutsideProject
		}
		if seen[dependency] {
			continue
		}
		seen[dependency] = true
		wanted = append(wanted, dependency)
	}

	edges[app.ID] = wanted
	if hasCycle(edges) {
		return ErrDependencyCycle
	}

	return db.WithTx(database, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM application_dependencies WHERE application_id = ?`, app.ID); err != nil {
			return fmt.Errorf("replace application dependencies: %w", err)
		}
		for _, dependency := range wanted {
			if _, err := tx.Exec(`INSERT INTO application_dependencies (application_id, depends_on_id)
				VALUES (?, ?)`, app.ID, dependency); err != nil {
				return fmt.Errorf("save application dependency: %w", err)
			}
		}
		return nil
	})
}

// hasCycle reports whether the dependency graph contains a cycle. Nodes are
// visited once: grey means "on the current path", black means "finished", so a
// grey node reached again closes a loop.
func hasCycle(edges map[string][]string) bool {
	const (
		grey  = 1
		black = 2
	)
	state := make(map[string]int, len(edges))

	var visit func(node string) bool
	visit = func(node string) bool {
		switch state[node] {
		case grey:
			return true
		case black:
			return false
		}
		state[node] = grey
		for _, next := range edges[node] {
			if visit(next) {
				return true
			}
		}
		state[node] = black
		return false
	}

	for node := range edges {
		if visit(node) {
			return true
		}
	}
	return false
}
