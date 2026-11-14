package settings

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// Service exposes settings over the application binding layer. It is a
// thin wrapper: validation and storage rules live in the package functions.
type Service struct {
	database *sql.DB
}

// NewService returns a settings service backed by database.
func NewService(database *sql.DB) *Service {
	return &Service{database: database}
}

// All returns every stored setting ordered by key.
func (s *Service) All() ([]Setting, error) {
	return All(s.database)
}

// Get returns the stored setting for key, or ErrNotFound.
func (s *Service) Get(key string) (Setting, error) {
	return Get(s.database, key)
}

// SetString stores a string setting.
func (s *Service) SetString(key string, value string) error {
	return SetString(s.database, key, value)
}

// SetBool stores a boolean setting.
func (s *Service) SetBool(key string, value bool) error {
	return SetBool(s.database, key, value)
}

// SetNumber stores a numeric setting.
func (s *Service) SetNumber(key string, value float64) error {
	return SetNumber(s.database, key, value)
}

// SetJSON stores a JSON setting; the payload must be valid JSON.
func (s *Service) SetJSON(key string, value json.RawMessage) error {
	var check any
	if err := json.Unmarshal(value, &check); err != nil {
		return fmt.Errorf("setting %q: invalid JSON: %w", key, err)
	}
	return Set(s.database, key, string(value), TypeJSON)
}

// Delete removes the setting for key. Deleting an absent key is not an
// error.
func (s *Service) Delete(key string) error {
	return Delete(s.database, key)
}

// Delete removes the setting for key.
func Delete(database *sql.DB, key string) error {
	_, err := database.Exec(`DELETE FROM settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}
	return nil
}
