package services

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"locafy.dev/desktop/internal/process"
)

// Events the supervisor emits, keyed by service id.
const (
	EventState  = "service:state"
	EventHealth = "service:health"
	EventOutput = "service:output"
)

// Timings for a supervised instance.
const (
	outputLines   = 500
	initTimeout   = 2 * time.Minute
	healthTimeout = 1500 * time.Millisecond
	// A cluster that just bound its port may still be in startup recovery, so
	// creating the project's database is retried a few times before it is
	// reported as a failure.
	databasePrepareAttempts = 8
	databasePrepareBackoff  = 800 * time.Millisecond
)

// A ticker and two ceilings, as variables so a test does not wait seconds for a
// reading or for a stopped process.
var (
	healthInterval = time.Second
	stopTimeout    = 25 * time.Second
	killTimeout    = 10 * time.Second
)

// healthProbe is a seam: tests answer without opening a socket.
var healthProbe = probePort

var staleRunMessage = "Locafy was closed while this service was running; start it again to bring it back."

// liveService is one supervised process and the output it has produced.
type liveService struct {
	done    chan struct{}
	pid     int
	lines   []string
	exited  bool
	stopped bool
	// prepared records that the project's database has been offered to the
	// cluster once this process reached the port.
	prepared bool
}

// Supervisor starts and stops service instances and records what happened. It
// keeps the last lines of output in memory while a process lives; durable logs
// belong to DESK-DEVX-001.
type Supervisor struct {
	database *sql.DB
	emit     func(name string, data any)

	mu   sync.Mutex
	runs map[string]*liveService
	// lastOutput keeps what a process said before it exited, so a failed start
	// can be read in the interface after the process is gone. Cleared when the
	// next run starts.
	lastOutput map[string][]string
}

// NewSupervisor returns a supervisor for the services in database. emit may be
// nil in tests.
func NewSupervisor(database *sql.DB, emit func(string, any)) *Supervisor {
	supervisor := &Supervisor{
		database:   database,
		emit:       emit,
		runs:       map[string]*liveService{},
		lastOutput: map[string][]string{},
	}
	supervisor.reconcile()
	return supervisor
}

// reconcile corrects records left running by a previous session: a process
// Locafy started is not adopted across restarts.
func (s *Supervisor) reconcile() {
	if s.database == nil {
		return
	}
	_, _ = s.database.Exec(`UPDATE services
		SET status = ?, error = ?, pid = NULL, health = '', health_detail = '', health_checked_at = ''
		WHERE status IN (?, ?, ?)`,
		StatusStopped, staleRunMessage, StatusStarting, StatusRunning, StatusStopping)
}

// Start initialises the data directory when it is empty, binds the recorded
// port and runs the engine in the foreground under supervision.
func (s *Supervisor) Start(id string) (Instance, error) {
	service, err := Get(s.database, id)
	if err != nil {
		return Instance{}, err
	}

	s.mu.Lock()
	_, running := s.runs[id]
	s.mu.Unlock()
	if running {
		return service, ErrAlreadyRunning
	}

	binaries, err := findPostgres()
	if err != nil {
		return service, s.fail(id, fmt.Errorf("%w: %s", ErrEngineUnavailable, missingEngineDetail(err)))
	}

	if err := prepareInstanceDirectory(service.DataDirectory); err != nil {
		return service, s.fail(id, err)
	}
	if err := initialiseData(service, binaries); err != nil {
		return service, s.fail(id, err)
	}
	if !portAvailable(service.Port) {
		return service, s.fail(id, fmt.Errorf("%w: port %d is held by another process; Locafy will not stop it",
			ErrPortInUse, service.Port))
	}

	argv := []string{
		"-D", service.DataDirectory,
		"-p", strconv.Itoa(service.Port),
		"-c", "listen_addresses=127.0.0.1",
	}
	command := process.CommandFor(context.Background(), binaries.postgres, argv)
	command.Dir = instanceDirectory(service.DataDirectory)
	command.Env = os.Environ()
	command.Stdin = nil

	stdout, err := command.StdoutPipe()
	if err != nil {
		return service, s.fail(id, fmt.Errorf("read output: %w", err))
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return service, s.fail(id, fmt.Errorf("read output: %w", err))
	}

	live := &liveService{done: make(chan struct{})}

	s.mu.Lock()
	if _, already := s.runs[id]; already {
		s.mu.Unlock()
		return service, ErrAlreadyRunning
	}
	s.runs[id] = live
	if err := command.Start(); err != nil {
		delete(s.runs, id)
		s.mu.Unlock()
		return service, s.fail(id, fmt.Errorf("start %s: %w", binaries.postgres, err))
	}
	live.pid = command.Process.Pid
	pid := live.pid
	// A new run starts with an empty transcript, so the panel never shows the
	// previous process's last words as if they belonged to this one.
	delete(s.lastOutput, id)
	s.setStatus(id, StatusRunning, "", &pid)
	s.mu.Unlock()

	s.emitState(id)
	go s.supervise(id, live, command, stdout, stderr)
	go s.trackHealth(id, live, service)

	started, err := Get(s.database, id)
	if err != nil {
		return service, nil
	}
	return started, nil
}

// supervise drains the engine's output, waits for the exit and records it.
func (s *Supervisor) supervise(id string, live *liveService, command *exec.Cmd, stdout io.ReadCloser, stderr io.ReadCloser) {
	var readers sync.WaitGroup
	readers.Add(2)
	stream := func(reader io.Reader) {
		defer readers.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r")
			s.mu.Lock()
			live.lines = append(live.lines, line)
			if len(live.lines) > outputLines {
				live.lines = live.lines[len(live.lines)-outputLines:]
			}
			s.mu.Unlock()
			if s.emit != nil {
				s.emit(EventOutput, map[string]any{"id": id, "line": line})
			}
		}
	}
	go stream(stdout)
	go stream(stderr)
	readers.Wait()

	waitErr := command.Wait()

	var exitCode *int
	if command.ProcessState != nil {
		if code := command.ProcessState.ExitCode(); code >= 0 {
			exitCode = &code
		}
	}

	s.mu.Lock()
	stopped := live.stopped
	live.exited = true
	output := append([]string(nil), live.lines...)
	s.lastOutput[id] = output
	delete(s.runs, id)
	s.mu.Unlock()

	status := StatusStopped
	message := ""
	if stopped {
		exitCode = nil
	} else if waitErr != nil {
		status = StatusError
		message = describeExit(waitErr, exitCode, output)
	}
	s.setStatus(id, status, message, nil)
	s.emitState(id)
	close(live.done)
}

// Stop asks the engine to shut down gracefully, then kills the tree if it does
// not comply.
func (s *Supervisor) Stop(id string) error {
	s.mu.Lock()
	live, ok := s.runs[id]
	if !ok || live.exited {
		s.mu.Unlock()
		return ErrNotRunning
	}
	live.stopped = true
	pid := live.pid
	s.mu.Unlock()

	s.setStatus(id, StatusStopping, "", nil)
	s.emitState(id)

	if service, err := Get(s.database, id); err == nil {
		requestShutdown(service)
	}

	select {
	case <-live.done:
		return nil
	case <-time.After(stopTimeout):
		if pid > 0 {
			process.KillTree(pid)
		}
	}
	select {
	case <-live.done:
		return nil
	case <-time.After(killTimeout):
		return fmt.Errorf("service process %d did not stop", pid)
	}
}

// Restart stops a running instance and starts it again from the same record,
// so the data directory and port are unchanged.
func (s *Supervisor) Restart(id string) (Instance, error) {
	if err := s.Stop(id); err != nil && !errors.Is(err, ErrNotRunning) {
		return Instance{}, err
	}
	return s.Start(id)
}

// StopAll kills every supervised instance so quitting Locafy leaves no server
// behind.
func (s *Supervisor) StopAll() {
	s.mu.Lock()
	instances := make([]Instance, 0, len(s.runs))
	pids := make([]int, 0, len(s.runs))
	for id, live := range s.runs {
		live.stopped = true
		if live.pid > 0 && !live.exited {
			pids = append(pids, live.pid)
		}
		if service, err := Get(s.database, id); err == nil {
			instances = append(instances, service)
		}
	}
	s.mu.Unlock()

	// Postgres and its siblings are given the chance to leave cleanly, because a
	// killed server needs crash recovery on the next start.
	for _, service := range instances {
		requestShutdown(service)
	}
	for _, pid := range pids {
		process.KillTree(pid)
	}
}

// Output returns the recent output of an instance, oldest first: the live
// process's lines while it runs, and what the last run said once it has exited.
func (s *Supervisor) Output(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if live, ok := s.runs[id]; ok {
		return append([]string(nil), live.lines...)
	}
	return append([]string(nil), s.lastOutput[id]...)
}

// trackHealth probes the port while the instance runs and records what it
// found. It never writes for a process that has already stopped.
func (s *Supervisor) trackHealth(id string, live *liveService, service Instance) {
	ticker := time.NewTicker(healthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-live.done:
			return
		case <-ticker.C:
			health := HealthHealthy
			detail := fmt.Sprintf("Answering on port %d.", service.Port)
			if err := healthProbe(service.Port); err != nil {
				health = HealthUnhealthy
				detail = fmt.Sprintf("No connection was accepted on port %d.", service.Port)
			} else if !live.prepared {
				live.prepared = true
				s.prepareDatabase(id, service)
			}
			if !s.setHealth(id, health, detail) {
				return
			}
			if s.emit != nil {
				s.emit(EventHealth, map[string]any{"id": id, "health": health, "detail": detail})
			}
		}
	}
}

// prepareDatabase makes sure the instance holds the database its connection
// details point at. The engine's own tool creates it, and an existing database
// is not an error. A cluster that has just bound its port may still be in
// startup recovery, so a refusal of that kind is retried rather than reported.
func (s *Supervisor) prepareDatabase(id string, service Instance) {
	binaries, err := findPostgres()
	if err != nil || binaries.createdb == "" {
		return
	}

	var output string
	var lastErr error
	for attempt := 0; attempt < databasePrepareAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), engineVersionTimeout)
		command := process.CommandFor(ctx, binaries.createdb, []string{
			"-h", "127.0.0.1",
			"-p", strconv.Itoa(service.Port),
			"-U", service.UserName,
			"-w",
			service.DatabaseName,
		})
		command.Env = os.Environ()
		result, err := command.CombinedOutput()
		cancel()
		output = string(result)
		lastErr = err
		if err == nil || strings.Contains(output, "already exists") {
			s.clearPrepareError(id)
			return
		}
		// A refusal with no output is as transient as "starting up": the tool
		// may have been busy or the connection dropped mid-handshake.
		if !stillStarting(output) && lastLines(output, 1) != "" {
			break
		}
		time.Sleep(databasePrepareBackoff)
	}

	detail := lastLines(output, 2)
	if detail == "" && lastErr != nil {
		detail = lastErr.Error()
	}
	message := fmt.Sprintf("The database %s could not be created: %s", service.DatabaseName, detail)
	if s.database != nil {
		_, _ = s.database.Exec(`UPDATE services SET error = ? WHERE id = ? AND status = ?`,
			message, id, StatusRunning)
	}
	s.emitState(id)
}

// stillStarting reports whether the engine refused work because the cluster has
// not finished coming up yet, which is worth retrying.
func stillStarting(output string) bool {
	for _, phrase := range []string{"starting up", "not yet accepting connections", "the database system is"} {
		if strings.Contains(output, phrase) {
			return true
		}
	}
	return false
}

// clearPrepareError removes a preparation failure once the database exists, so a
// retry that succeeded does not leave a stale warning on the record.
func (s *Supervisor) clearPrepareError(id string) {
	if s.database == nil {
		return
	}
	_, _ = s.database.Exec(`UPDATE services SET error = ''
		WHERE id = ? AND status = ? AND error LIKE 'The database % could not be created%'`, id, StatusRunning)
}

// initialiseData runs the engine's initialiser once, when the data directory
// has no cluster in it yet.
func initialiseData(service Instance, binaries postgresBinaries) error {
	if _, err := os.Stat(filepath.Join(service.DataDirectory, "PG_VERSION")); err == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()

	argv := []string{
		"-D", service.DataDirectory,
		"-U", service.UserName,
		"--auth=trust",
		"--encoding=UTF8",
		"--locale=C",
	}
	command := process.CommandFor(ctx, binaries.initdb, argv)
	command.Dir = instanceDirectory(service.DataDirectory)
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if err != nil {
		tail := lastLines(string(output), 3)
		if tail == "" {
			return fmt.Errorf("initialise the data directory: %w", err)
		}
		return fmt.Errorf("initialise the data directory: %w: %s", err, tail)
	}
	return nil
}

// requestShutdown asks the engine's control program to stop the cluster. It is
// best effort: the caller falls back to killing the supervised tree.
func requestShutdown(service Instance) {
	binaries, err := findPostgres()
	if err != nil {
		return
	}
	// A data directory without a cluster in it has nothing to stop.
	if _, err := os.Stat(filepath.Join(service.DataDirectory, "PG_VERSION")); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	command := process.CommandFor(ctx, binaries.pgctl,
		[]string{"stop", "-D", service.DataDirectory, "-m", "fast", "-w", "-t", "20"})
	command.Dir = instanceDirectory(service.DataDirectory)
	command.Env = os.Environ()
	_ = command.Run()
}

// probePort reports whether anything accepts a connection on the port. It
// dials, closes and writes nothing.
func probePort(port int) error {
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	connection, err := net.DialTimeout("tcp", address, healthTimeout)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", address, err)
	}
	return connection.Close()
}

// fail records a start that did not happen, so the interface can say why.
func (s *Supervisor) fail(id string, err error) error {
	s.setStatus(id, StatusError, err.Error(), nil)
	s.emitState(id)
	return err
}

// setStatus writes a process state. A state that begins or ends a run also
// resets the health reading, so a stopped instance never shows a stale probe.
func (s *Supervisor) setStatus(id string, status string, message string, pid *int) {
	if s.database == nil {
		return
	}
	_, _ = s.database.Exec(`UPDATE services SET status = ?, error = ?, pid = ? WHERE id = ?`,
		status, message, pid, id)
	switch status {
	case StatusRunning:
		_, _ = s.database.Exec(`UPDATE services
			SET started_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
				health = '', health_detail = '', health_checked_at = ''
			WHERE id = ?`, id)
	case StatusStopped, StatusError:
		_, _ = s.database.Exec(`UPDATE services
			SET health = '', health_detail = '', health_checked_at = ''
			WHERE id = ?`, id)
	}
}

// setHealth records a probe result, but only while the instance is running.
// It reports whether the write applied.
func (s *Supervisor) setHealth(id string, health string, detail string) bool {
	if s.database == nil {
		return false
	}
	result, err := s.database.Exec(`UPDATE services
		SET health = ?, health_detail = ?, health_checked_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ? AND status = ?`, health, detail, id, StatusRunning)
	if err != nil {
		return false
	}
	affected, err := result.RowsAffected()
	return err == nil && affected == 1
}

// emitState announces the record's current state.
func (s *Supervisor) emitState(id string) {
	if s.emit == nil {
		return
	}
	service, err := Get(s.database, id)
	if err != nil {
		return
	}
	s.emit(EventState, service)
}

// describeExit turns a failed exit into a sentence, keeping the engine's own
// last words where it produced any.
func describeExit(err error, exitCode *int, output []string) string {
	var message string
	switch {
	case exitCode != nil:
		message = fmt.Sprintf("The service stopped with exit code %d.", *exitCode)
	default:
		message = fmt.Sprintf("The service stopped: %v.", err)
	}
	if tail := lastLines(strings.Join(output, "\n"), 3); tail != "" {
		message += " " + tail
	}
	return message
}

// lastLines returns the last count non-empty lines of text, joined by a space.
func lastLines(text string, count int) string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, " ")
}
