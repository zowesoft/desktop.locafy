package downloads

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"locafy.dev/desktop/internal/db"
)

// jobColumns are the columns every job read shares.
const jobColumns = `id, kind_id, label, state, progress, total, error, target_dir, created_at, updated_at`

func scanJob(scanner interface{ Scan(...any) error }) (Job, error) {
	var job Job
	if err := scanner.Scan(&job.ID, &job.KindID, &job.Label, &job.State, &job.Progress,
		&job.Total, &job.Error, &job.TargetDir, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return Job{}, err
	}
	return job, nil
}

// now is the timestamp shape the schema's defaults use.
func now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// insertJob records a fresh queued row with a known id, so the caller can
// read exactly the row it created without racing another insert.
func (m *Manager) insertJob(label string, spec Spec) (Job, error) {
	id := db.NewID()
	encoded, err := json.Marshal(spec)
	if err != nil {
		return Job{}, fmt.Errorf("encode install spec: %w", err)
	}
	target := filepath.Join(m.root, spec.Kind, spec.Version)
	if _, err := m.db.Exec(`INSERT INTO install_jobs (id, kind_id, label, spec, state, target_dir)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, spec.Kind, label, string(encoded), StateQueued, target); err != nil {
		return Job{}, fmt.Errorf("record install job: %w", err)
	}
	return m.Get(id)
}

// Get reads one job row.
func (m *Manager) Get(id string) (Job, error) {
	job, err := scanJob(m.db.QueryRow(`SELECT `+jobColumns+` FROM install_jobs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("read install job: %w", err)
	}
	return job, nil
}

// List reads every job, newest first.
func (m *Manager) List() ([]Job, error) {
	rows, err := m.db.Query(`SELECT ` + jobColumns + ` FROM install_jobs ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("read install jobs: %w", err)
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("read install job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// setState writes a state and publishes the row.
func (m *Manager) setState(id string, state string) {
	m.publish(id, `UPDATE install_jobs SET state = ?, updated_at = ? WHERE id = ?`, state, now())
}

// setProgress records the downloaded bytes and publishes the row.
func (m *Manager) setProgress(id string, progress int64, total int64) {
	m.publish(id, `UPDATE install_jobs SET progress = ?, total = ?, updated_at = ? WHERE id = ?`, progress, total, now())
}

func (m *Manager) fail(id string, err error) {
	m.publish(id, `UPDATE install_jobs SET state = ?, error = ?, updated_at = ? WHERE id = ?`, StateFailed, err.Error(), now())
}

func (m *Manager) finish(id string, state string) {
	m.publish(id, `UPDATE install_jobs SET state = ?, error = '', updated_at = ? WHERE id = ?`, state, now())
}

// publish runs a row write and emits the changed row. A failed publish is
// not an error: the row is the truth and the view will catch up.
func (m *Manager) publish(id string, query string, args ...any) {
	// Every row query names its row last, so callers argue only about the
	// columns they change.
	if _, err := m.db.Exec(query, append(args, id)...); err != nil {
		return
	}
	if job, err := m.Get(id); err == nil && m.emit != nil {
		m.emit(EventJob, job)
	}
}

// Cancel interrupts a running job. The download loop notices the cancelled
// context, removes its temporary file and marks the row cancelled; the
// calling goroutine marks that state when the context fires.
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	cancel, running := m.live[id]
	m.mu.Unlock()
	if !running {
		return ErrRunning
	}
	cancel()
	return nil
}

// Retry re-runs a failed or cancelled job as a fresh attempt, so the history
// keeps both. A queued, running or succeeded job is refused.
func (m *Manager) Retry(id string) (Job, error) {
	job, err := m.Get(id)
	if err != nil {
		return Job{}, err
	}
	switch job.State {
	case StateFailed, StateCancelled:
	default:
		return Job{}, ErrNotTerminal
	}

	var spec Spec
	var specJSON string
	if err := m.db.QueryRow(`SELECT spec FROM install_jobs WHERE id = ?`, id).Scan(&specJSON); err != nil {
		return Job{}, fmt.Errorf("read install job: %w", err)
	}
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		return Job{}, fmt.Errorf("read install spec: %w", err)
	}

	fresh, err := m.insertJob(job.Label, spec)
	if err != nil {
		return Job{}, err
	}
	m.spawn(fresh)
	return fresh, nil
}

// Remove takes a finished job out of the history. A queued or running job is
// refused; the files an install produced are never touched.
func (m *Manager) Remove(id string) error {
	job, err := m.Get(id)
	if err != nil {
		return err
	}
	switch job.State {
	case StateFailed, StateCancelled, StateSucceeded:
	default:
		return ErrRunning
	}
	if _, err := m.db.Exec(`DELETE FROM install_jobs WHERE id = ?`, id); err != nil {
		return fmt.Errorf("remove install job: %w", err)
	}
	return nil
}
