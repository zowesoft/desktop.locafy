// Package machine maintains the single machine record describing the
// computer Locafy is installed on.
package machine

import (
	"database/sql"
	"fmt"
	"os"
	"runtime"
)

const recordID = "local"

// Register creates or refreshes the machine record. It runs at engine
// startup, so the record always reflects the current host.
func Register(database *sql.DB) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("read hostname: %w", err)
	}
	version, err := osVersion()
	if err != nil {
		return fmt.Errorf("read OS version: %w", err)
	}

	_, err = database.Exec(`INSERT INTO machine (id, hostname, platform, architecture, os_version)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			hostname = excluded.hostname,
			platform = excluded.platform,
			architecture = excluded.architecture,
			os_version = excluded.os_version`,
		recordID, hostname, runtime.GOOS, runtime.GOARCH, version)
	if err != nil {
		return fmt.Errorf("upsert machine record: %w", err)
	}
	return nil
}
