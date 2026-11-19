package downloads

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Job states. Queued means registered but not yet running; the state lives
// in the row so the history survives a restart.
const (
	StateQueued    = "queued"
	StateRunning   = "running"
	StateSucceeded = "succeeded"
	StateFailed    = "failed"
	StateCancelled = "cancelled"
)

// Job is one install attempt, machine-scoped and retained in history.
type Job struct {
	ID        string `json:"id"`
	KindID    string `json:"kindId"`
	Label     string `json:"label"`
	State     string `json:"state"`
	Progress  int64  `json:"progress"`
	Total     int64  `json:"total"`
	Error     string `json:"error"`
	TargetDir string `json:"targetDir"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// EventJob is the event name the manager emits whenever a job row changes.
const EventJob = "downloads:jobs"

// ErrNotFound, ErrRunning and ErrNotTerminal are the refusals the service
// returns for jobs the caller names wrongly.
var (
	ErrNotFound    = errors.New("that install job does not exist")
	ErrRunning     = errors.New("that install job is still running")
	ErrNotTerminal = errors.New("only a finished job can be retried")
	ErrNoInstall   = errors.New("Locafy has no installable for that runtime on this machine")
	ErrInstalled   = errors.New("that version is already installed")
)

// Manager runs install jobs in the background, one row per attempt, with a
// log file each. It is the download twin of the scaffold manager.
type Manager struct {
	mu     sync.Mutex
	db     *sql.DB
	root   string
	emit   func(string, any)
	client *http.Client
	live   map[string]context.CancelFunc
}

// NewManager returns a downloads manager that unpacks into root. emit
// receives EventJob for every row change; it may be nil in tests.
func NewManager(db *sql.DB, root string, emit func(string, any)) *Manager {
	return &Manager{
		db:     db,
		root:   root,
		emit:   emit,
		client: &http.Client{Timeout: 30 * time.Minute},
		live:   map[string]context.CancelFunc{},
	}
}

// Start installs a kind's catalogue entry in the background and returns the
// fresh job.
func (m *Manager) Start(kindID string) (Job, error) {
	installable, ok := Find(kindID)
	if !ok {
		return Job{}, ErrNoInstall
	}
	target := filepath.Join(m.root, installable.Spec.Kind, installable.Spec.Version)
	if _, err := os.Stat(target); err == nil {
		return Job{}, ErrInstalled
	}

	job, err := m.insertJob(installable.Label, installable.Spec)
	if err != nil {
		return Job{}, err
	}
	m.spawn(job)
	return job, nil
}

// spawn runs a queued job in the background, marking it running first so a
// restart never leaves a queued job forever.
func (m *Manager) spawn(job Job) {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.live[job.ID] = cancel
	m.mu.Unlock()

	m.setState(job.ID, StateRunning)
	go func() {
		defer m.setStateIfLive(job.ID, StateCancelled)
		m.run(ctx, job)
	}()
}

// setStateIfLive writes a terminal state only when no newer state stands: a
// failed row is never overwritten by the cancel that follows the context.
func (m *Manager) setStateIfLive(id string, state string) {
	job, err := m.Get(id)
	if err != nil || job.State != StateRunning {
		return
	}
	m.setState(id, state)
}

// run does the work: download to a temporary file with progress, verify the
// archive, unpack it into the target, then succeed or fail the row.
func (m *Manager) run(ctx context.Context, job Job) {
	defer func() {
		m.mu.Lock()
		delete(m.live, job.ID)
		m.mu.Unlock()
	}()

	var spec Spec
	var specJSON string
	if err := m.db.QueryRow(`SELECT spec FROM install_jobs WHERE id = ?`, job.ID).Scan(&specJSON); err != nil {
		m.fail(job.ID, fmt.Errorf("read install job: %w", err))
		return
	}
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		m.fail(job.ID, fmt.Errorf("read install spec: %w", err))
		return
	}

	downloadErr := func() error {
		archive, cleanup, err := m.download(ctx, job.ID, spec)
		if err != nil {
			return err
		}
		defer cleanup()

		if err := unpack(archive, spec, job.TargetDir); err != nil {
			return err
		}
		m.finish(job.ID, StateSucceeded)
		return nil
	}()
	if downloadErr == nil {
		return
	}
	// A cancelled context is a cancelled job, not a failure: the operator
	// stopped it.
	if ctx.Err() != nil {
		m.setState(job.ID, StateCancelled)
		return
	}
	m.fail(job.ID, downloadErr)
}

// download streams the archive into a temporary file, updating the row's
// progress as bytes arrive so the view can show a real fraction.
func (m *Manager) download(ctx context.Context, id string, spec Spec) (string, func(), error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.URL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("prepare the download: %w", err)
	}
	response, err := m.client.Do(request)
	if err != nil {
		return "", nil, fmt.Errorf("download %s: %w", spec.URL, err)
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return "", nil, fmt.Errorf("download %s: the server answered %s", spec.URL, response.Status)
	}

	temp, err := os.CreateTemp("", "locafy-install-*")
	if err != nil {
		response.Body.Close()
		return "", nil, fmt.Errorf("make a temporary file: %w", err)
	}

	// Progress is throttled: a row update per chunk would flood the view, so
	// the counter is flushed at most a few times a second.
	var written int64
	var lastFlush time.Time
	buffer := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			temp.Close()
			response.Body.Close()
			os.Remove(temp.Name())
			return "", nil, ctx.Err()
		default:
		}
		n, err := response.Body.Read(buffer)
		if n > 0 {
			if _, err := temp.Write(buffer[:n]); err != nil {
				temp.Close()
				response.Body.Close()
				os.Remove(temp.Name())
				return "", nil, fmt.Errorf("store the archive: %w", err)
			}
			written += int64(n)
			if time.Since(lastFlush) > 250*time.Millisecond {
				lastFlush = time.Now()
				m.setProgress(id, written, response.ContentLength)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			temp.Close()
			response.Body.Close()
			os.Remove(temp.Name())
			return "", nil, fmt.Errorf("download the archive: %w", err)
		}
	}
	temp.Close()
	response.Body.Close()
	m.setProgress(id, written, response.ContentLength)

	path := temp.Name()
	return path, func() { os.Remove(path) }, nil
}

// unpack verifies the archive's shape and unpacks it into target, refusing a
// member that would escape the directory.
func unpack(archive string, spec Spec, target string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("make the install directory: %w", err)
	}
	switch spec.Format {
	case "zip":
		return unpackZip(archive, target, spec)
	case "targz":
		return unpackTarGz(archive, target, spec)
	}
	return fmt.Errorf("unknown archive format %q", spec.Format)
}

func unpackZip(archive string, target string, spec Spec) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open the archive: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		name, ok := stripRoot(file.Name, spec.Strip)
		if !ok || name == "" {
			continue
		}
		destination, ok := safeJoin(target, name)
		if !ok {
			return fmt.Errorf("the archive names a file outside the install directory: %s", name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return fmt.Errorf("make a folder: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return fmt.Errorf("make a folder: %w", err)
		}
		if err := copyMember(func() (io.Reader, error) { return file.Open() }, destination, file.Modified); err != nil {
			return err
		}
	}
	return verifyContains(target, spec.MustContain)
}

func unpackTarGz(archive string, target string, spec Spec) error {
	file, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("open the archive: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open the archive: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read the archive: %w", err)
		}
		name, ok := stripRoot(header.Name, spec.Strip)
		if !ok || name == "" {
			continue
		}
		destination, ok := safeJoin(target, name)
		if !ok {
			return fmt.Errorf("the archive names a file outside the install directory: %s", name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return fmt.Errorf("make a folder: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return fmt.Errorf("make a folder: %w", err)
			}
			mode := os.FileMode(header.Mode)
			if err := copyMember(func() (io.Reader, error) { return tarReader, nil }, destination, mode); err != nil {
				return err
			}
		}
	}
	return verifyContains(target, spec.MustContain)
}

// copyMember writes one archive member to disk with its permissions.
func copyMember(open func() (io.Reader, error), destination string, mode any) error {
	source, err := open()
	if err != nil {
		return fmt.Errorf("read an archive member: %w", err)
	}
	defer func() {
		if closer, ok := source.(io.Closer); ok {
			closer.Close()
		}
	}()

	writer, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode(mode))
	if err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	if _, err := io.Copy(writer, source); err != nil {
		writer.Close()
		return fmt.Errorf("write %s: %w", destination, err)
	}
	return writer.Close()
}

func fileMode(mode any) os.FileMode {
	if m, ok := mode.(os.FileMode); ok {
		result := m.Perm()
		if m&0o100 != 0 {
			result |= 0o111
		}
		return result
	}
	return 0o755
}

// stripRoot removes the archive's wrapping folders. A member inside them
// keeps its relative name; the wrapping folder itself contributes nothing.
func stripRoot(name string, strip int) (string, bool) {
	name = filepath.ToSlash(strings.TrimPrefix(name, "./"))
	if strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
		return "", false
	}
	parts := strings.Split(name, "/")
	if len(parts) <= strip {
		return "", true
	}
	return strings.Join(parts[strip:], "/"), true
}

// safeJoin refuses a destination that would leave the target directory.
func safeJoin(target string, name string) (string, bool) {
	destination := filepath.Join(target, filepath.FromSlash(name))
	cleaned := filepath.Clean(destination)
	if !strings.HasPrefix(cleaned, filepath.Clean(target)+string(os.PathSeparator)) {
		return "", false
	}
	return cleaned, true
}

// verifyContains proves the unpack worked: the expected executable exists
// where the catalogue says it should.
func verifyContains(target string, mustContain string) error {
	if mustContain == "" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(mustContain))); err != nil {
		return fmt.Errorf("the archive did not produce %s", mustContain)
	}
	return nil
}
