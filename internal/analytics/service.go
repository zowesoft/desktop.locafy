package analytics

import "database/sql"

// Service exposes the recorded history over the binding layer.
type Service struct {
	database *sql.DB
}

// NewService returns an analytics service backed by database.
func NewService(database *sql.DB) *Service {
	return &Service{database: database}
}

// Recent returns the newest recorded actions, machine scope included.
func (s *Service) Recent(limit int) ([]Event, error) {
	return RecentEvents(s.database, limit)
}

// Trend returns the machine-wide activity history for the last days days.
func (s *Service) Trend(days int) (Trend, error) {
	return TrendDays(s.database, days)
}

// Retention returns how many days of history the engine keeps.
func (s *Service) Retention() (int, error) {
	return Retention(s.database)
}

// SetRetention stores the retention preference and trims the history to it.
func (s *Service) SetRetention(days int) error {
	return SetRetention(s.database, days)
}
