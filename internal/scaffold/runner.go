package scaffold

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"locafy.dev/desktop/internal/process"
)

// Job states, matching the scaffold_jobs CHECK constraint.
const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
	JobCancelled = "cancelled"
)

// Job is a scaffold run: what it will do, where it writes and how it ended.
type Job struct {
	ID            string
	ProjectID     string
	RecipeID      string
	Directory     string
	TargetPath    string
	Status        string
	Error         string
	LogPath       string
	Commands      []string
	CreatedTarget bool
	CreatedAt     string
	UpdatedAt     string
}

// RunSpec is everything the runner needs; the service has already resolved the
// target and bound every argument.
type RunSpec struct {
	ProjectID     string
	RecipeID      string
	Directory     string
	Target        string
	Steps         []Step
	Verify        []string
	CreatedTarget bool
	OnSuccess     func(Job) error
}

type liveJob struct {
	job    Job
	lines  []string
	cancel context.CancelFunc
	pid    int
}

// Manager runs scaffolds one job at a time in the background, streaming their
// output. It keeps live state in memory and, when a database is present, a row
// per job so the history survives closing the dialog.
type Manager struct {
	mu      sync.Mutex
	jobs    map[string]*liveJob
	db      *sql.DB
	logDir  string
	emit    func(name string, data any)
	timeout time.Duration
}

// NewManager returns a runner. emit receives "scaffold:output" and
// "scaffold:state" events; it may be nil in tests.
func NewManager(db *sql.DB, logDir string, emit func(string, any)) *Manager {
	return &Manager{
		jobs:    map[string]*liveJob{},
		db:      db,
		logDir:  logDir,
		emit:    emit,
		timeout: 20 * time.Minute,
	}
}

// Start registers a job and runs it in the background. It returns as soon as
// the first command is queued.
func (m *Manager) Start(spec RunSpec) (Job, error) {
	id := newJobID()
	job := Job{
		ID:            id,
		ProjectID:     spec.ProjectID,
		RecipeID:      spec.RecipeID,
		Directory:     spec.Directory,
		TargetPath:    spec.Target,
		Status:        JobQueued,
		CreatedTarget: spec.CreatedTarget,
	}
	for _, step := range spec.Steps {
		job.Commands = append(job.Commands, step.Display)
	}
	if m.logDir != "" {
		job.LogPath = filepath.Join(m.logDir, "scaffold-"+id+".log")
	}

	m.mu.Lock()
	m.jobs[id] = &liveJob{job: job, lines: []string{}}
	m.mu.Unlock()
	m.persist(job)
	m.emitState(job)

	go m.run(id, spec)
	return job, nil
}

func (m *Manager) run(id string, spec RunSpec) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	m.mu.Lock()
	if live, ok := m.jobs[id]; ok {
		live.cancel = cancel
		live.job.Status = JobRunning
		m.persistLocked(live.job)
		m.mu.Unlock()
		m.emitState(live.job)
	} else {
		m.mu.Unlock()
	}

	logFile := m.openLog(id)
	if logFile != nil {
		defer logFile.Close()
	}

	for _, step := range spec.Steps {
		if err := m.runStep(ctx, id, spec.Target, step, logFile); err != nil {
			status := JobFailed
			if errors.Is(ctx.Err(), context.Canceled) {
				status = JobCancelled
			}
			m.finish(id, status, err.Error())
			return
		}
	}

	missing, ok := DirectoriesReady(spec.Target, spec.Verify)
	if !ok {
		m.finish(id, JobFailed, "the installer finished but "+strings.Join(missing, ", ")+" is missing")
		return
	}

	m.mu.Lock()
	live := m.jobs[id]
	m.mu.Unlock()
	job := live.job
	if spec.OnSuccess != nil {
		if err := spec.OnSuccess(job); err != nil {
			m.finish(id, JobFailed, "the app was created but could not be registered: "+err.Error())
			return
		}
	}
	m.finish(id, JobSucceeded, "")
}

func (m *Manager) runStep(ctx context.Context, id string, target string, step Step, logFile *os.File) error {
	command := process.CommandFor(ctx, step.Bin, step.Args)
	if step.Cwd == "inside" {
		command.Dir = target
	} else {
		command.Dir = filepath.Dir(target)
	}
	command.Env = append(os.Environ(),
		"CI=1",
		"COMPOSER_NO_INTERACTION=1",
		"npm_config_yes=true",
		"NO_COLOR=1",
		"FORCE_COLOR=0",
	)
	command.Stdin = nil

	m.line(id, "$ "+step.Display, logFile)

	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("read output: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return fmt.Errorf("read output: %w", err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start %s: %w", step.Bin, err)
	}
	m.rememberProcess(id, command)

	done := make(chan struct{}, 2)
	stream := func(scanner *bufio.Scanner) {
		for scanner.Scan() {
			m.line(id, scanner.Text(), logFile)
		}
		done <- struct{}{}
	}
	go stream(bufio.NewScanner(stdout))
	go stream(bufio.NewScanner(stderr))
	<-done
	<-done

	if err := command.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%s failed: %w", step.Bin, err)
	}
	return nil
}

func (m *Manager) openLog(id string) *os.File {
	if m.logDir == "" {
		return nil
	}
	if err := os.MkdirAll(m.logDir, 0o755); err != nil {
		return nil
	}
	file, err := os.OpenFile(filepath.Join(m.logDir, "scaffold-"+id+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil
	}
	return file
}

func (m *Manager) line(id string, text string, logFile *os.File) {
	text = strings.TrimRight(text, "\r")
	m.mu.Lock()
	if live, ok := m.jobs[id]; ok {
		live.lines = append(live.lines, text)
		if len(live.lines) > 4000 {
			live.lines = live.lines[len(live.lines)-4000:]
		}
	}
	m.mu.Unlock()
	if logFile != nil {
		_, _ = logFile.WriteString(text + "\n")
	}
	if m.emit != nil {
		m.emit("scaffold:output", map[string]any{"jobId": id, "line": text})
	}
}

func (m *Manager) rememberProcess(id string, command *exec.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if live, ok := m.jobs[id]; ok {
		live.pid = command.Process.Pid
	}
}

func (m *Manager) finish(id string, status string, message string) {
	m.mu.Lock()
	live, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	live.job.Status = status
	live.job.Error = message
	m.persistLocked(live.job)
	job := live.job
	lines := append([]string(nil), live.lines...)
	m.mu.Unlock()

	if m.emit != nil {
		m.emit("scaffold:output", map[string]any{"jobId": id, "line": terminalLine(status, message)})
		m.emit("scaffold:state", map[string]any{"jobId": id, "status": status, "error": message})
	}
	_ = lines
	_ = job
}

func terminalLine(status string, message string) string {
	switch status {
	case JobSucceeded:
		return "Done."
	case JobCancelled:
		return "Cancelled."
	default:
		if message == "" {
			return "Failed."
		}
		return "Failed: " + message
	}
}

// Get returns a job and its recent output.
func (m *Manager) Get(id string) (Job, []string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	live, ok := m.jobs[id]
	if !ok {
		return Job{}, nil, fmt.Errorf("scaffold job not found")
	}
	return live.job, append([]string(nil), live.lines...), nil
}

// Cancel stops a running job and its child processes.
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	live, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("scaffold job not found")
	}
	if live.job.Status != JobRunning && live.job.Status != JobQueued {
		return nil
	}
	pid := live.pid
	if live.cancel != nil {
		live.cancel()
	}
	if pid > 0 {
		process.KillTree(pid)
	}
	return nil
}

// Forget drops a finished job from memory.
func (m *Manager) Forget(id string) {
	m.mu.Lock()
	delete(m.jobs, id)
	m.mu.Unlock()
}

func (m *Manager) emitState(job Job) {
	if m.emit == nil {
		return
	}
	m.emit("scaffold:state", map[string]any{"jobId": job.ID, "status": job.Status, "error": job.Error})
}

func (m *Manager) persist(job Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.persistLocked(job)
}

func (m *Manager) persistLocked(job Job) {
	if m.db == nil {
		return
	}
	_, err := m.db.Exec(`INSERT INTO scaffold_jobs
			(id, project_id, recipe_id, directory, target_path, status, error, log_path, commands)
		VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id = NULLIF(excluded.project_id, ''),
			status = excluded.status,
			error = excluded.error,
			log_path = excluded.log_path,
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
		job.ID, job.ProjectID, job.RecipeID, job.Directory, job.TargetPath, job.Status,
		job.Error, job.LogPath, strings.Join(job.Commands, "\n"))
	if err != nil {
		// History is a convenience; a failure here must not stop the install.
		return
	}
}

func newJobID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
}

// Discard removes a failed scaffold's folder when Locafy created it, never
// when the destination already existed.
func Discard(job Job) error {
	if job.Status == JobSucceeded || !job.CreatedTarget {
		return errors.New("only a failed scaffold's own folder can be removed")
	}
	cleaned := filepath.Clean(job.TargetPath)
	if cleaned == "" || cleaned == string(filepath.Separator) {
		return errors.New("refusing to remove that path")
	}
	return os.RemoveAll(cleaned)
}
