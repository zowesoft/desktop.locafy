package downloads

// Service exposes the install jobs over the binding layer. The manager runs
// the work; the service is the seam the application talks to.
type Service struct {
	manager *Manager
}

// NewService returns a downloads service. root is the directory Locafy
// unpacks managed runtimes into.
func NewService(manager *Manager) *Service {
	return &Service{manager: manager}
}

// Installables lists everything this machine can install, so the view can
// offer the buttons that start a job.
func (s *Service) Installables() []Installable {
	return Catalogue()
}

// List reports every install job, newest first.
func (s *Service) List() ([]Job, error) {
	return s.manager.List()
}

// Install starts a kind's catalogue entry as a background job.
func (s *Service) Install(kindID string) (Job, error) {
	return s.manager.Start(kindID)
}

// Retry re-runs a failed or cancelled job as a fresh attempt.
func (s *Service) Retry(id string) (Job, error) {
	return s.manager.Retry(id)
}

// Cancel interrupts a running job.
func (s *Service) Cancel(id string) error {
	return s.manager.Cancel(id)
}

// Remove takes a finished job out of the history.
func (s *Service) Remove(id string) error {
	return s.manager.Remove(id)
}
