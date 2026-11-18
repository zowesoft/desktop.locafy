package applications

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"locafy.dev/desktop/internal/process"
	"locafy.dev/desktop/internal/projects"
)

// Process states the applications table already allows besides "stopped".
const (
	StatusStarting = "starting"
	StatusRunning  = "running"
	StatusStopping = "stopping"
	StatusError    = "error"
)

// Events the supervisor emits, keyed by application id.
const (
	EventOutput = "application:output"
	EventState  = "application:state"
	EventPorts  = "application:ports"
)

// outputLines is how much of a live process's output stays in memory for a
// view that opens late. Durable logs belong to DESK-DEVX-001.
const outputLines = 500

// stopTimeout is how long Stop waits for a killed tree to report its exit.
const stopTimeout = 10 * time.Second

// portScanInterval is how often a running application's listening ports are
// re-read. Servers bind shortly after they start and may open more sockets
// later, so one reading at start time would miss them.
const portScanInterval = 2 * time.Second

const staleRunMessage = "Locafy was closed while this application was running; start it again to bring it back."

var (
	ErrNoStartCommand      = errors.New("this application has no start command")
	ErrInvalidStartCommand = errors.New("the start command cannot be parsed into a program and its arguments")
	ErrExecutableNotFound  = errors.New("the start command's program was not found")
	ErrAlreadyRunning      = errors.New("this application is already running")
	ErrNotRunning          = errors.New("this application is not running")
	ErrPortInUse           = errors.New("the application's port is already in use")
)

// run is one live process: the output collected so far, the ports it was seen
// listening on, and the channel its supervisor closes once it has exited.
type run struct {
	done    chan struct{}
	pid     int
	name    string
	lines   []string
	ports   []int
	stopped bool
	exited  bool
}

// Supervisor runs saved applications one process at a time and records what
// happened. It owns no file handles and writes no logs: output is streamed and
// the last lines stay in memory while the process lives.
type Supervisor struct {
	database *sql.DB
	emit     func(name string, data any)

	mu   sync.Mutex
	runs map[string]*run
}

// NewSupervisor returns a supervisor for applications stored in database. emit
// receives EventOutput and EventState; it may be nil in tests.
func NewSupervisor(database *sql.DB, emit func(string, any)) *Supervisor {
	supervisor := &Supervisor{database: database, emit: emit, runs: map[string]*run{}}
	supervisor.reconcile()
	return supervisor
}

// reconcile corrects records left in a process state by a previous run: the
// process Locafy started is not tracked across restarts.
func (s *Supervisor) reconcile() {
	if s.database == nil {
		return
	}
	_, _ = s.database.Exec(`UPDATE applications
		SET status = ?, error = ?, exit_code = NULL
		WHERE status IN (?, ?, ?)`,
		StatusStopped, staleRunMessage, StatusStarting, StatusRunning, StatusStopping)
}

// Start runs an application's stored command in its stored folder. It returns
// once the process has started, not when it exits.
func (s *Supervisor) Start(id string) (Application, error) {
	app, err := Get(s.database, id)
	if err != nil {
		return Application{}, err
	}

	if strings.TrimSpace(app.StartCommand) == "" {
		return app, s.fail(app.ID, ErrNoStartCommand)
	}

	project, err := projects.Get(s.database, app.ProjectID)
	if err != nil {
		return Application{}, err
	}
	directory := filepath.Join(project.RootPath, filepath.FromSlash(app.Directory))
	info, err := os.Stat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return app, s.fail(app.ID, ErrDirectoryMissing)
	}
	if err != nil {
		return app, s.fail(app.ID, fmt.Errorf("inspect %q: %w", directory, err))
	}
	if !info.IsDir() {
		return app, s.fail(app.ID, ErrDirectoryNotDir)
	}

	// The last port this application used is the only port Locafy can talk
	// about before a new process binds one.
	if conflict := s.portConflict(app); conflict != "" {
		return app, s.failConflict(app.ID, conflict)
	}

	argv, err := splitCommand(app.StartCommand)
	if err != nil {
		return app, s.fail(app.ID, fmt.Errorf("%w: %s", ErrInvalidStartCommand, app.StartCommand))
	}
	bin, err := resolveExecutable(argv[0], directory)
	if err != nil {
		return app, s.fail(app.ID, err)
	}

	command := process.CommandFor(context.Background(), bin, argv[1:])
	command.Dir = directory
	// A development server should see the same environment as a terminal.
	command.Env = os.Environ()
	command.Stdin = nil

	stdout, err := command.StdoutPipe()
	if err != nil {
		return app, s.fail(app.ID, fmt.Errorf("read output: %w", err))
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return app, s.fail(app.ID, fmt.Errorf("read output: %w", err))
	}

	live := &run{done: make(chan struct{}), name: app.Name}

	s.mu.Lock()
	if _, running := s.runs[id]; running {
		s.mu.Unlock()
		return app, ErrAlreadyRunning
	}
	s.runs[id] = live
	if err := command.Start(); err != nil {
		delete(s.runs, id)
		s.mu.Unlock()
		return app, s.fail(app.ID, fmt.Errorf("start %s: %w", bin, err))
	}
	live.pid = command.Process.Pid
	// Written under the lock so a Stop cannot race the transition to running.
	s.update(app.ID, StatusRunning, nil, "", true)
	s.mu.Unlock()

	s.emitState(app.ID, StatusRunning, nil, "")
	go s.supervise(app.ID, live, command, stdout, stderr)
	go s.trackPorts(app.ID, live, live.pid)

	started, err := Get(s.database, app.ID)
	if err != nil {
		return app, nil
	}
	return started, nil
}

// supervise drains the process's output, waits for it to exit and records the
// outcome.
func (s *Supervisor) supervise(id string, live *run, command *exec.Cmd, stdout io.ReadCloser, stderr io.ReadCloser) {
	var readers sync.WaitGroup
	readers.Add(2)
	stream := func(reader io.Reader) {
		defer readers.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			s.appendLine(id, live, strings.TrimRight(scanner.Text(), "\r"))
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
	delete(s.runs, id)
	s.mu.Unlock()
	close(live.done)

	status := StatusStopped
	message := ""
	if !stopped && waitErr != nil {
		status = StatusError
		message = describeExit(waitErr, exitCode)
	}
	if stopped {
		// A kill's exit code is not a failure the operator needs to read back.
		exitCode = nil
	}
	s.update(id, status, exitCode, message, false)
	s.emitState(id, status, exitCode, message)
}

// Stop kills the application's process tree and waits for the exit.
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

	s.update(id, StatusStopping, nil, "", false)
	s.emitState(id, StatusStopping, nil, "")

	if pid > 0 {
		process.KillTree(pid)
	}
	select {
	case <-live.done:
		return nil
	case <-time.After(stopTimeout):
		return fmt.Errorf("application process %d did not stop", pid)
	}
}

// Restart stops an application if it is running and starts it again.
func (s *Supervisor) Restart(id string) (Application, error) {
	if err := s.Stop(id); err != nil && !errors.Is(err, ErrNotRunning) {
		return Application{}, err
	}
	return s.Start(id)
}

// StopAll kills every supervised process tree. Quitting Locafy calls it so no
// server outlives the window that started it.
func (s *Supervisor) StopAll() {
	s.mu.Lock()
	pids := make([]int, 0, len(s.runs))
	for _, live := range s.runs {
		live.stopped = true
		if live.pid > 0 && !live.exited {
			pids = append(pids, live.pid)
		}
	}
	s.mu.Unlock()

	for _, pid := range pids {
		process.KillTree(pid)
	}
}

// Output returns the recent output of a live process, oldest first.
func (s *Supervisor) Output(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	live, ok := s.runs[id]
	if !ok {
		return nil
	}
	return append([]string(nil), live.lines...)
}

// portConflict explains why an application cannot start: the port it listened
// on last time is held. It never stops the holder — that is the operator's
// call, and the holder may not even be Locafy's to touch.
func (s *Supervisor) portConflict(app Application) string {
	if app.Port == nil {
		return ""
	}
	port := *app.Port

	listeners, err := listeningFor()
	if err != nil {
		// A scan that cannot answer is not a conflict.
		return ""
	}
	holder, found := listenerOn(port, listeners)
	if !found {
		return ""
	}

	if owner := s.supervisedApplication(holder.PID); owner != "" {
		return fmt.Sprintf("Port %d is already used by %s, another Locafy application. Stop it first.", port, owner)
	}
	who := holder.Process
	if who == "" {
		who = "another program"
	}
	return fmt.Sprintf("Port %d is already used by %s (PID %d), which Locafy did not start. Locafy will not stop it; free the port yourself, then start this application again.", port, who, holder.PID)
}

// supervisedApplication names the Locafy application whose process tree holds
// pid, or returns empty when the process is not one of ours.
func (s *Supervisor) supervisedApplication(pid int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, live := range s.runs {
		if live.pid > 0 && ownedBy(pid, live.pid) {
			return live.name
		}
	}
	return ""
}

// failConflict records a refused start as a port problem, not an app failure.
func (s *Supervisor) failConflict(id string, message string) error {
	if s.database != nil {
		_, _ = s.database.Exec(`UPDATE applications
			SET status = ?, error = '', exit_code = NULL, port_conflict = ?
			WHERE id = ?`, StatusError, message, id)
	}
	if s.emit != nil {
		s.emit(EventState, map[string]any{
			"applicationId": id,
			"status":        StatusError,
			"exitCode":      nil,
			"error":         "",
			"portConflict":  message,
		})
	}
	return fmt.Errorf("%w: %s", ErrPortInUse, message)
}

// trackPorts records the ports the tree listens on for as long as it lives.
func (s *Supervisor) trackPorts(id string, live *run, root int) {
	ticker := time.NewTicker(portScanInterval)
	defer ticker.Stop()
	for {
		s.scanPorts(id, live, root)
		select {
		case <-live.done:
			return
		case <-ticker.C:
		}
	}
}

func (s *Supervisor) scanPorts(id string, live *run, root int) {
	listeners, err := listeningFor()
	if err != nil {
		return
	}
	ports := portsOwnedBy(listeners, root)
	if len(ports) == 0 {
		// Nothing observed yet: keep the last reading rather than flapping to
		// "no port" between a server's binds.
		return
	}

	s.mu.Lock()
	changed := !samePorts(live.ports, ports)
	if changed {
		live.ports = ports
	}
	s.mu.Unlock()
	if !changed {
		return
	}

	s.updatePort(id, ports[0])
	if s.emit != nil {
		s.emit(EventPorts, map[string]any{"applicationId": id, "ports": ports})
	}
}

func (s *Supervisor) updatePort(id string, port int) {
	if s.database == nil {
		return
	}
	_, _ = s.database.Exec(`UPDATE applications SET port = ? WHERE id = ?`, port, id)
}

func samePorts(left []int, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (s *Supervisor) appendLine(id string, live *run, line string) {
	s.mu.Lock()
	live.lines = append(live.lines, line)
	if len(live.lines) > outputLines {
		live.lines = live.lines[len(live.lines)-outputLines:]
	}
	s.mu.Unlock()

	if s.emit != nil {
		s.emit(EventOutput, map[string]any{"applicationId": id, "line": line})
	}
}

// fail records why an application could not start and returns the reason.
func (s *Supervisor) fail(id string, err error) error {
	s.update(id, StatusError, nil, err.Error(), false)
	s.emitState(id, StatusError, nil, err.Error())
	return err
}

func (s *Supervisor) update(id string, status string, exitCode *int, message string, starting bool) {
	if s.database == nil {
		return
	}
	var code any
	if exitCode != nil {
		code = *exitCode
	}
	if starting {
		_, _ = s.database.Exec(`UPDATE applications
			SET status = ?, exit_code = NULL, error = '', port_conflict = '',
				started_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
			WHERE id = ?`, status, id)
		return
	}
	_, _ = s.database.Exec(`UPDATE applications SET status = ?, exit_code = ?, error = ? WHERE id = ?`,
		status, code, message, id)
}

func (s *Supervisor) emitState(id string, status string, exitCode *int, message string) {
	if s.emit == nil {
		return
	}
	s.emit(EventState, map[string]any{
		"applicationId": id,
		"status":        status,
		"exitCode":      exitCode,
		"error":         message,
	})
}

// describeExit turns a wait error into something an operator can act on.
func describeExit(err error, exitCode *int) string {
	if exitCode != nil {
		return fmt.Sprintf("Exited with code %d.", *exitCode)
	}
	return err.Error()
}

// splitCommand turns a stored start command into an argv. It is quote-aware and
// never invokes a shell, so the operator's command is exactly what runs. There
// are no escape sequences: a backslash is a path separator, so the Windows path
// in a stored command survives unchanged.
func splitCommand(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	hasToken := false

	flush := func() {
		if hasToken {
			args = append(args, current.String())
			current.Reset()
			hasToken = false
		}
	}

	for _, char := range command {
		if quote != 0 {
			if char == quote {
				quote = 0
				continue
			}
			current.WriteRune(char)
			hasToken = true
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
			hasToken = true
		case ' ', '\t', '\r', '\n':
			flush()
		default:
			current.WriteRune(char)
			hasToken = true
		}
	}

	if quote != 0 {
		return nil, ErrInvalidStartCommand
	}
	flush()
	if len(args) == 0 {
		return nil, ErrInvalidStartCommand
	}
	return args, nil
}

// resolveExecutable finds the program a start command names. A path is taken
// from the application's folder; a bare name goes through PATH, which on
// Windows also resolves the .cmd shims a package manager installs.
func resolveExecutable(name string, directory string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(directory, path)
		}
		path = filepath.Clean(path)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return "", fmt.Errorf("%w: %s", ErrExecutableNotFound, path)
		}
		return path, nil
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%w on PATH: %s", ErrExecutableNotFound, name)
	}
	return path, nil
}
