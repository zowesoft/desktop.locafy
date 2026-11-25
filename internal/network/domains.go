package network

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"locafy.dev/desktop/internal/db"
)

// DevelopmentSuffix is the reserved suffix for local domains (RFC 2606 reserves
// .test for exactly this). A user-configured suffix is later work.
const DevelopmentSuffix = ".test"

var (
	// ErrInvalidHostname is returned for a name that is not a usable local
	// domain: wrong suffix, bad label, a port or a path in it.
	ErrInvalidHostname = errors.New("a local domain needs a hostname like api.project.test")
	// ErrReservedHostname refuses names Locafy reserves for cloud and tunnel
	// addresses, which are never local domains (PRD section 11.1).
	ErrReservedHostname = errors.New("locafy.dev hostnames are reserved for cloud and tunnel addresses")
	// ErrHostnameClaimed is returned when another live domain already uses the
	// hostname, in any project.
	ErrHostnameClaimed = errors.New("that hostname is already assigned")
	// ErrDomainNotFound is returned when a domain is gone.
	ErrDomainNotFound = errors.New("domain not found")
	// ErrUnknownApplication is returned when the application does not belong to
	// the project the domain is being assigned in.
	ErrUnknownApplication = errors.New("application not found in this project")
)

// Domain is one local hostname routed to an application.
type Domain struct {
	ID              string `json:"id"`
	ProjectID       string `json:"projectID"`
	ApplicationID   string `json:"applicationID"`
	ApplicationName string `json:"applicationName"`
	Hostname        string `json:"hostname"`
	// Port is the application's discovered port, nil until it has bound one.
	// A domain without a port is a valid record that cannot route yet.
	Port      *int   `json:"port"`
	CreatedAt string `json:"createdAt"`
}

// NormalizeHostname lowercases and validates a local domain. The value it
// returns is what is stored and what the proxy routes on, so there is one
// spelling of a hostname everywhere.
func NormalizeHostname(raw string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(raw))
	host = strings.TrimSuffix(host, ".")

	if host == "" || strings.ContainsAny(host, "/\\:?#@ ") {
		return "", ErrInvalidHostname
	}
	if host == "locafy.dev" || strings.HasSuffix(host, ".locafy.dev") {
		return "", ErrReservedHostname
	}
	if !strings.HasSuffix(host, DevelopmentSuffix) || host == DevelopmentSuffix {
		return "", ErrInvalidHostname
	}

	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "", ErrInvalidHostname
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return "", ErrInvalidHostname
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", ErrInvalidHostname
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return "", ErrInvalidHostname
			}
		}
	}
	return host, nil
}

// List returns a project's domains with the name and port of the application
// each one routes to.
func List(database *sql.DB, projectID string) ([]Domain, error) {
	rows, err := database.Query(`SELECT d.id, d.project_id, d.application_id, a.name, d.hostname,
			a.port, d.created_at
		FROM domains d
		JOIN applications a ON a.id = d.application_id
		WHERE d.project_id = ? AND d.deleted_at IS NULL AND a.deleted_at IS NULL
		ORDER BY d.hostname`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	domains := make([]Domain, 0)
	for rows.Next() {
		var domain Domain
		var port sql.NullInt64
		if err := rows.Scan(&domain.ID, &domain.ProjectID, &domain.ApplicationID,
			&domain.ApplicationName, &domain.Hostname, &port, &domain.CreatedAt); err != nil {
			return nil, err
		}
		if port.Valid {
			value := int(port.Int64)
			domain.Port = &value
		}
		domains = append(domains, domain)
	}
	return domains, rows.Err()
}

// Assign attaches a hostname to an application in the project. The hostname is
// validated and normalised here, and a name another live domain already holds is
// refused rather than silently taken over.
func Assign(database *sql.DB, projectID string, applicationID string, rawHostname string) (Domain, error) {
	hostname, err := NormalizeHostname(rawHostname)
	if err != nil {
		return Domain{}, err
	}

	var name string
	err = database.QueryRow(`SELECT name FROM applications
		WHERE id = ? AND project_id = ? AND deleted_at IS NULL`, applicationID, projectID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return Domain{}, ErrUnknownApplication
	}
	if err != nil {
		return Domain{}, err
	}

	id := db.NewID()
	_, err = database.Exec(`INSERT INTO domains (id, project_id, application_id, hostname)
		VALUES (?, ?, ?, ?)`, id, projectID, applicationID, hostname)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Domain{}, ErrHostnameClaimed
		}
		return Domain{}, fmt.Errorf("assign %s: %w", hostname, err)
	}

	domains, err := List(database, projectID)
	if err != nil {
		return Domain{}, err
	}
	for _, domain := range domains {
		if domain.ID == id {
			return domain, nil
		}
	}
	return Domain{}, fmt.Errorf("assign %s: the record was not readable back", hostname)
}

// Remove releases a hostname. The name becomes available to any project, and the
// application is untouched.
func Remove(database *sql.DB, id string) error {
	result, err := database.Exec(`UPDATE domains
		SET deleted_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrDomainNotFound
	}
	return nil
}
