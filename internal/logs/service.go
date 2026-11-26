package logs

// Service exposes the captured output over the binding layer. Capture itself
// happens in the supervisors; this is the read side the interface uses.
type Service struct {
	store *Store
}

// NewService returns a logs service reading from store.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// Entries returns captured output lines oldest first. A filter without a
// ProjectID covers every project.
func (s *Service) Entries(filter Filter) ([]Entry, error) {
	return s.store.Query(filter), nil
}
