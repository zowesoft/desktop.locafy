package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// NewID returns a new UUID for schema records.
func NewID() string {
	return uuid.NewString()
}

// WithTx runs fn inside a transaction, committing on success and rolling
// back when fn returns an error. Rollback after a failed commit is ignored;
// any other rollback failure is reported with the original error attached.
func WithTx(database *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback after error (%v): %w", err, rbErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
