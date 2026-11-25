package network

import (
	"database/sql"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Service exposes local domains and the proxy's state to the interface. Storage
// rules live in domains.go; the listener lives in proxy.go.
type Service struct {
	database *sql.DB
	proxy    *Proxy
}

// NewService returns a network service backed by database.
func NewService(database *sql.DB) *Service {
	service := &Service{database: database}
	service.proxy = NewProxy(database, func(name string, data any) {
		if app := application.Get(); app != nil {
			app.Event.Emit(name, data)
		}
	})
	return service
}

// List returns a project's domains with the application each one routes to.
func (s *Service) List(projectID string) ([]Domain, error) {
	return List(s.database, projectID)
}

// Assign gives an application a local hostname, refusing a name that is
// invalid, reserved or already claimed. The routing table is rebuilt at once.
func (s *Service) Assign(projectID string, applicationID string, hostname string) (Domain, error) {
	domain, err := Assign(s.database, projectID, applicationID, hostname)
	if err != nil {
		return Domain{}, err
	}
	s.proxy.Reload()
	return domain, nil
}

// Remove releases a hostname and rebuilds the routing table.
func (s *Service) Remove(id string) error {
	if err := Remove(s.database, id); err != nil {
		return err
	}
	s.proxy.Reload()
	return nil
}

// StartProxy binds the local listener. A refused port is reported in the status
// with its reason.
func (s *Service) StartProxy() (ProxyStatus, error) {
	return s.proxy.Start(DefaultAddress)
}

// StopProxy closes the listener.
func (s *Service) StopProxy() ProxyStatus {
	return s.proxy.Stop()
}

// ProxyStatus reports the listener, the route count and the last failure.
func (s *Service) ProxyStatus() ProxyStatus {
	return s.proxy.Status()
}

// ReloadRoutes rebuilds the routing table from the current records, for an
// application whose port changed while the interface was open.
func (s *Service) ReloadRoutes() ProxyStatus {
	return s.proxy.Reload()
}
