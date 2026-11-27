// Package logs keeps the recent output of every process Locafy started, so the
// Logs screen and each project's Logs tab read one attributed stream instead of
// the process panel's own buffer. Lines are masked before they are stored.
package logs

import (
	"database/sql"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Layers name the kind of process a line came from.
const (
	LayerApplication = "application"
	LayerService     = "service"
)

// Levels are read from the line's wording, the only signal an unstructured
// process output offers. They drive the severity column and filter, so a line
// that names a failure is shown as one instead of being buried in the stream.
const (
	LevelError = "error"
	LevelWarn  = "warn"
	LevelInfo  = "info"
	LevelDebug = "debug"
)

// timeFormat keeps milliseconds, the precision a log reader expects to see.
const timeFormat = "2006-01-02T15:04:05.000Z07:00"

// DefaultCapacity is how many lines the store keeps, oldest dropped first.
const DefaultCapacity = 2000

// Entry is one captured output line.
type Entry struct {
	ID       int64  `json:"id"`
	Time     string `json:"time"`
	Layer    string `json:"layer"`
	SourceID string `json:"sourceId"`
	Source   string `json:"source"`
	Level    string `json:"level"`
	Line     string `json:"line"`
	Masked   bool   `json:"masked"`
}

// Filter narrows a query. An empty field matches everything; Limit caps the
// number of entries returned, newest kept.
type Filter struct {
	ProjectID string `json:"projectId"`
	Layer     string `json:"layer"`
	SourceID  string `json:"sourceId"`
	Level     string `json:"level"`
	Contains  string `json:"contains"`
	Limit     int    `json:"limit"`
}

// Store holds the recent lines of every source in a bounded ring. It is safe for
// concurrent use: the supervisors write from their drain goroutines and the
// binding reads from the interface's.
type Store struct {
	database *sql.DB
	capacity int

	mu      sync.Mutex
	entries []Entry
	nextID  int64
}

// NewStore returns a store that keeps capacity lines. A nil database is allowed
// in tests: project filtering simply matches nothing then.
func NewStore(database *sql.DB, capacity int) *Store {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Store{database: database, capacity: capacity}
}

// Record captures a line from one source. A nil store ignores the call, so a
// supervisor built without a log sink keeps working in tests.
func (s *Store) Record(layer string, sourceID string, line string) {
	if s == nil || strings.TrimSpace(line) == "" {
		return
	}
	clean, masked := Mask(line)
	entry := Entry{
		Time:     time.Now().UTC().Format(timeFormat),
		Layer:    layer,
		SourceID: sourceID,
		Level:    LevelOf(clean),
		Line:     clean,
		Masked:   masked,
	}

	s.mu.Lock()
	s.nextID++
	entry.ID = s.nextID
	s.entries = append(s.entries, entry)
	if len(s.entries) > s.capacity {
		s.entries = s.entries[len(s.entries)-s.capacity:]
	}
	s.mu.Unlock()
}

// Query returns the matching lines oldest first, the order a terminal shows
// them, with each entry's source name filled in from the records.
func (s *Store) Query(filter Filter) []Entry {
	if s == nil {
		return nil
	}
	limit := filter.Limit
	if limit <= 0 || limit > s.capacity {
		limit = 200
	}
	allowed := s.projectSources(filter.ProjectID)
	names := s.sourceNames()
	needle := strings.ToLower(strings.TrimSpace(filter.Contains))

	s.mu.Lock()
	defer s.mu.Unlock()

	found := make([]Entry, 0, limit)
	for index := len(s.entries) - 1; index >= 0 && len(found) < limit; index-- {
		entry := s.entries[index]
		if filter.Layer != "" && entry.Layer != filter.Layer {
			continue
		}
		if filter.Level != "" && entry.Level != filter.Level {
			continue
		}
		if filter.SourceID != "" && entry.SourceID != filter.SourceID {
			continue
		}
		if allowed != nil && !allowed[entry.Layer+"/"+entry.SourceID] {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(entry.Line), needle) {
			continue
		}
		entry.Source = names[entry.Layer+"/"+entry.SourceID]
		found = append(found, entry)
	}

	for left, right := 0, len(found)-1; left < right; left, right = left+1, right-1 {
		found[left], found[right] = found[right], found[left]
	}
	return found
}

// projectSources names the layer/source pairs that belong to a project, or nil
// when the query is not scoped to one.
func (s *Store) projectSources(projectID string) map[string]bool {
	if projectID == "" || s.database == nil {
		return nil
	}
	allowed := map[string]bool{}
	for _, source := range []struct {
		layer string
		query string
	}{
		{LayerApplication, `SELECT id FROM applications WHERE project_id = ? AND deleted_at IS NULL`},
		{LayerService, `SELECT id FROM services WHERE project_id = ? AND deleted_at IS NULL`},
	} {
		rows, err := s.database.Query(source.query, projectID)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				allowed[source.layer+"/"+id] = true
			}
		}
		rows.Close()
	}
	return allowed
}

// sourceNames maps every recorded application and service to its name.
func (s *Store) sourceNames() map[string]string {
	names := map[string]string{}
	if s.database == nil {
		return names
	}
	for _, source := range []struct {
		layer string
		query string
	}{
		{LayerApplication, `SELECT id, name FROM applications WHERE deleted_at IS NULL`},
		{LayerService, `SELECT id, name FROM services WHERE deleted_at IS NULL`},
	} {
		rows, err := s.database.Query(source.query)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err == nil {
				names[source.layer+"/"+id] = name
			}
		}
		rows.Close()
	}
	return names
}

// LevelOf reads a line's severity from its wording.
func LevelOf(line string) string {
	switch {
	case errorLine.MatchString(line):
		return LevelError
	case warnLine.MatchString(line):
		return LevelWarn
	case debugLine.MatchString(line):
		return LevelDebug
	default:
		return LevelInfo
	}
}

// Mask replaces values that look like secrets, so a password printed by a
// framework never reaches the log store, the interface or an exported copy. It
// reports whether it changed the line.
func Mask(line string) (string, bool) {
	masked := authorizationHeader.ReplaceAllString(line, "${1}"+redaction)
	masked = urlCredentials.ReplaceAllString(masked, "${1}"+redaction+"@")
	masked = assignments.ReplaceAllString(masked, "${1}${2}"+redaction)
	masked = bearer.ReplaceAllString(masked, "${1}"+redaction)
	return masked, masked != line
}

const redaction = "•••"

var (
	// A whole Authorization header, scheme and token alike.
	authorizationHeader = regexp.MustCompile(`(?i)(\bauthorization\s*[:=]\s*)[^\r\n]+`)
	// name=value and name: value pairs for the usual credential words. The
	// word carries no boundary requirement: env-style names (DB_PASSWORD)
	// hide the keyword behind an underscore, and masking too much is the safe
	// direction.
	assignments = regexp.MustCompile(`(?i)(password|passwd|pwd|secret|client[_-]?secret|token|api[_-]?key|access[_-]?key|private[_-]?key)(\s*[:=]\s*)([^\s"',;&]+)`)
	// Bearer tokens in request headers and logs.
	bearer = regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9._~+/=-]{8,}`)
	// Credentials embedded in a URL.
	urlCredentials = regexp.MustCompile(`(://[^/\s:@]+:)[^/\s@]+@`)
	// Severity words a line may name itself with.
	errorLine = regexp.MustCompile(`(?i)\b(err|error|fatal|panic|exception|traceback|failed|failure|refused|denied)\b`)
	warnLine  = regexp.MustCompile(`(?i)\b(warn|warning|deprecated|slow|timeout|timed out|retry|retrying|caution)\b`)
	debugLine = regexp.MustCompile(`(?i)\b(debug|trace|verbose)\b`)
)
