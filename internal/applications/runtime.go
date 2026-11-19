package applications

import (
	"database/sql"
	"fmt"
	"strings"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/runtimes"
)

// SetRuntime pins the runtime copy an application's start resolves first, or
// clears the pin with an empty id. The id must name a file that exists, so an
// assignment the machine can no longer honour is refused here as well as at
// start time.
func SetRuntime(database *sql.DB, applicationID string, runtimeID string) error {
	app, err := Get(database, applicationID)
	if err != nil {
		return err
	}
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID != "" {
		if _, err := runtimes.Resolve(runtimeID); err != nil {
			return err
		}
	}
	if _, err := database.Exec(`UPDATE applications SET runtime_id = ? WHERE id = ?`, runtimeID, applicationID); err != nil {
		return fmt.Errorf("store application runtime: %w", err)
	}
	action := "Runtime cleared"
	if runtimeID != "" {
		action = "Runtime assigned"
	}
	return activity.Log(database, activity.ScopeProject, app.ProjectID, action, app.Name)
}
