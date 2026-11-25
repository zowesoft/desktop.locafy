package network

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Service exposes local domains, the proxy's state and the certificate trust
// step to the interface. Storage rules live in domains.go, the listeners in
// proxy.go and the certificates in certs.go.
type Service struct {
	database *sql.DB
	proxy    *Proxy
}

// CertificatesDirectory is where Locafy keeps its local authority, beside the
// engine's data rather than inside the project.
func CertificatesDirectory() string {
	config, err := os.UserConfigDir()
	if err != nil {
		config = os.TempDir()
	}
	return filepath.Join(config, "Locafy", "certs")
}

// NewService returns a network service backed by database.
func NewService(database *sql.DB) *Service {
	service := &Service{database: database}
	service.proxy = NewProxy(database, NewAuthority(CertificatesDirectory()), func(name string, data any) {
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

// StartProxy binds the local listeners. Ports that cannot be bound fall back to
// their unprivileged alternatives; the status reports what was actually bound.
func (s *Service) StartProxy() (ProxyStatus, error) {
	return s.proxy.Start(DefaultHTTP, DefaultHTTPS)
}

// StopProxy closes both listeners.
func (s *Service) StopProxy() ProxyStatus {
	return s.proxy.Stop()
}

// ProxyStatus reports the listeners, the route count and the last failure.
func (s *Service) ProxyStatus() ProxyStatus {
	return s.proxy.Status()
}

// ReloadRoutes rebuilds the routing table and, when the domain set changed, the
// certificate.
func (s *Service) ReloadRoutes() ProxyStatus {
	return s.proxy.Reload()
}

// TrustPlan is what the interface shows before it offers to trust the
// certificate: the exact command, and the explanation of what it changes.
type TrustPlan struct {
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
	Certificate string `json:"certificate"`
}

// TrustPlan describes the platform command that trusts Locafy's authority. It
// runs nothing.
func (s *Service) TrustPlan() (TrustPlan, error) {
	program, args, explanation, err := s.proxy.TrustCommand()
	if err != nil {
		return TrustPlan{Certificate: s.proxy.CAPath()}, err
	}
	return TrustPlan{
		Command:     strings.Join(append([]string{program}, args...), " "),
		Explanation: explanation,
		Certificate: s.proxy.CAPath(),
	}, nil
}

// TrustCertificate runs the trust command the plan named and returns its output.
// The user asks for this; nothing is installed silently.
func (s *Service) TrustCertificate() (string, error) {
	program, args, explanation, err := s.proxy.TrustCommand()
	if err != nil {
		return "", err
	}

	command := exec.Command(program, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return message, fmt.Errorf("%s failed: %s", program, message)
	}
	return explanation, nil
}
