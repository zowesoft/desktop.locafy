// Package settings stores and retrieves typed application preferences.
package settings

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// ValueType enumerates the supported setting value types.
type ValueType string

const (
	TypeString  ValueType = "string"
	TypeBoolean ValueType = "boolean"
	TypeNumber  ValueType = "number"
	TypeJSON    ValueType = "json"
)

// ErrNotFound is returned when no setting exists for the requested key.
var ErrNotFound = errors.New("setting not found")

// Setting is a stored preference with its value and declared type.
type Setting struct {
	Key   string
	Value string
	Type  ValueType
}

// Set stores a setting value with an explicit type, replacing any
// previous value for the key.
func Set(database *sql.DB, key string, value string, valueType ValueType) error {
	_, err := database.Exec(`INSERT INTO settings (key, value, value_type) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			value_type = excluded.value_type`,
		key, value, valueType)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// Get returns the stored setting for key, or ErrNotFound.
func Get(database *sql.DB, key string) (Setting, error) {
	var setting Setting
	err := database.QueryRow(`SELECT key, value, value_type FROM settings WHERE key = ?`, key).
		Scan(&setting.Key, &setting.Value, &setting.Type)
	if errors.Is(err, sql.ErrNoRows) {
		return Setting{}, ErrNotFound
	}
	if err != nil {
		return Setting{}, fmt.Errorf("get setting %q: %w", key, err)
	}
	return setting, nil
}

// All returns every stored setting ordered by key.
func All(database *sql.DB) ([]Setting, error) {
	rows, err := database.Query(`SELECT key, value, value_type FROM settings ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()

	var result []Setting
	for rows.Next() {
		var setting Setting
		if err := rows.Scan(&setting.Key, &setting.Value, &setting.Type); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		result = append(result, setting)
	}
	return result, rows.Err()
}

// SetString stores a string setting.
func SetString(database *sql.DB, key string, value string) error {
	return Set(database, key, value, TypeString)
}

// String returns the value of a string-typed setting.
func String(database *sql.DB, key string) (string, error) {
	setting, err := Get(database, key)
	if err != nil {
		return "", err
	}
	if setting.Type != TypeString {
		return "", fmt.Errorf("setting %q has type %q, want %q", key, setting.Type, TypeString)
	}
	return setting.Value, nil
}

// SetBool stores a boolean setting.
func SetBool(database *sql.DB, key string, value bool) error {
	return Set(database, key, strconv.FormatBool(value), TypeBoolean)
}

// Bool returns the value of a boolean-typed setting.
func Bool(database *sql.DB, key string) (bool, error) {
	setting, err := Get(database, key)
	if err != nil {
		return false, err
	}
	if setting.Type != TypeBoolean {
		return false, fmt.Errorf("setting %q has type %q, want %q", key, setting.Type, TypeBoolean)
	}
	value, err := strconv.ParseBool(setting.Value)
	if err != nil {
		return false, fmt.Errorf("setting %q: %w", key, err)
	}
	return value, nil
}

// SetNumber stores a numeric setting as its shortest decimal form.
func SetNumber(database *sql.DB, key string, value float64) error {
	return Set(database, key, strconv.FormatFloat(value, 'g', -1, 64), TypeNumber)
}

// Number returns the value of a numeric-typed setting.
func Number(database *sql.DB, key string) (float64, error) {
	setting, err := Get(database, key)
	if err != nil {
		return 0, err
	}
	if setting.Type != TypeNumber {
		return 0, fmt.Errorf("setting %q has type %q, want %q", key, setting.Type, TypeNumber)
	}
	value, err := strconv.ParseFloat(setting.Value, 64)
	if err != nil {
		return 0, fmt.Errorf("setting %q: %w", key, err)
	}
	return value, nil
}

// SetJSON stores a JSON setting; the value is validated by marshalling.
func SetJSON(database *sql.DB, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("setting %q: encode JSON: %w", key, err)
	}
	return Set(database, key, string(encoded), TypeJSON)
}

// JSON decodes the value of a JSON-typed setting into dest.
func JSON(database *sql.DB, key string, dest any) error {
	setting, err := Get(database, key)
	if err != nil {
		return err
	}
	if setting.Type != TypeJSON {
		return fmt.Errorf("setting %q has type %q, want %q", key, setting.Type, TypeJSON)
	}
	if err := json.Unmarshal([]byte(setting.Value), dest); err != nil {
		return fmt.Errorf("setting %q: decode JSON: %w", key, err)
	}
	return nil
}
