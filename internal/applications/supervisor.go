package applications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"locafy.dev/desktop/internal/network"
	"locafy.dev/desktop/internal/process"
	"locafy.dev/desktop/internal/projects"
	"locafy.dev/desktop/internal/runtimes"
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
	EventHealth = "application:health"
)

// outputLines is how much of a live process's output stays in memory for a
// view that opens late. Durable logs belong to DESK-DEVX-001.
const outputLines = 500

// stopTimeout is how long Stop waits for a killed tree to report its exit. It is
// a variable so a test can shorten the wait.
var stopTimeout = 10 * time.Second

// stopPoll is how often Stop asks whether the process itself has gone, rather
// than trusting only its supervisor's exit record.
const stopPoll = 50 * time.Millisecond

// processAlive is the platform's liveness check, overridable in tests.
var processAlive = process.Alive

// runtimeScanInterval is how often a running application's ports and health
// are re-read. Servers bind shortly after they start and may open sockets
// later, so one reading at start time would miss them.
const runtimeScanInterval = 2 * time.Second

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
	done         chan struct{}
	pid          int
	name         string
	lines        []string
	ports        []int
	health       string
	healthDetail string
	usage        Usage
	stopped      bool
	exited       bool
	finalised    bool
}

// Supervisor runs saved applications one process at a time and records what
// happened. It owns no file handles and writes no logs: output is streamed and
// the last lines stay in memory while the process lives.
type Supervisor struct {
	database *sql.DB
	emit     func(name string, data any)

	mu   sync.Mutex
	runs map[string]*run
	// busy marks projects mid-orchestration so only one project action runs
	// at a time.
	busy map[string]string
	// cpuBaseline holds each tree's last CPU sample for the usage delta, keyed
	// by the tree's root pid.
	cpuBaseline map[int]cpuTimes
	// services runs the backing services a project start brings up first. It
	// is nil when the project has none wired, as in most tests.
	services ServiceRuntime
}

// NewSupervisor returns a supervisor for applications stored in database. emit
// receives EventOutput and EventState; it may be nil in tests.
func NewSupervisor(database *sql.DB, emit func(string, any)) *Supervisor {
	supervisor := &Supervisor{
		database:    database,
		emit:        emit,
		runs:        map[string]*run{},
		cpuBaseline: map[int]cpuTimes{},
	}
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

	// An assigned runtime must still be on the machine: the start resolves
	// its directory ahead of the machine PATH and refuses when it is gone.
	env := os.Environ()
	runtimeDir := ""
	if strings.TrimSpace(app.RuntimeID) != "" {
		copy, err := runtimes.Resolve(app.RuntimeID)
		if err != nil {
			return app, s.fail(app.ID, fmt.Errorf("the assigned runtime is not available: %w", err))
		}
		runtimeDir = filepath.Dir(copy.Path)
		env = prependPath(env, runtimeDir)
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
	bin, err := resolveExecutableIn(argv[0], directory, runtimeDir)
	if err != nil {
		return app, s.fail(app.ID, err)
	}

	command := process.CommandFor(context.Background(), bin, argv[1:])
	command.Dir = directory
	// A development server should see the same environment as a terminal,
	// with an assigned runtime's directory ahead of the machine PATH.
	//
	// It also needs Locafy's certificate: a Next.js server fetching another
	// Locafy domain ignores the Windows trust store, and the alternative would
	// be a wrapper script in every project (Node reads NODE_EXTRA_CA_CERTS).
	command.Env = network.CertificateEnvironment(env)
	command.Stdin = nil

	outputs, err := process.NewOutputs(command)
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
		outputs.Close()
		delete(s.runs, id)
		s.mu.Unlock()
		return app, s.fail(app.ID, fmt.Errorf("start %s: %w", bin, err))
	}
	outputs.Release()
	live.pid = command.Process.Pid
	// Written under the lock so a Stop cannot race the transition to running.
	s.update(app.ID, StatusRunning, nil, "", true)
	s.mu.Unlock()

	s.emitState(app.ID, StatusRunning, nil, "")
	go s.supervise(app.ID, live, command, outputs.Stdout, outputs.Stderr)
	go s.trackRuntime(app.ID, live, live.pid)

	started, err := Get(s.database, app.ID)
	if err != nil {
		return app, nil
	}
	return started, nil
}

// supervise drains the process's output, waits for it to exit and records the
// outcome.
func (s *Supervisor) supervise(id string, live *run, command *exec.Cmd, stdout io.ReadCloser, stderr io.ReadCloser) {
	exited := make(chan struct{})
	var readers sync.WaitGroup
	readers.Add(2)
	stream := func(reader io.ReadCloser) {
		defer readers.Done()
		process.Stream(reader, exited, func(line string) {
			s.appendLine(id, live, line)
		})
	}
	go stream(stdout)
	go stream(stderr)

	waitErr := command.Wait()
	close(exited)
	readers.Wait()

	var exitCode *int
	if command.ProcessState != nil {
		if code := command.ProcessState.ExitCode(); code >= 0 {
			exitCode = &code
		}
	}

	s.mu.Lock()
	stopped := live.stopped
	s.mu.Unlock()

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
	s.finish(id, live, status, exitCode, message)
}

// finish records a process's terminal state exactly once and releases whoever
// is waiting. Usually the supervising goroutine gets here first; a Stop that saw
// the process itself disappear can get here earlier, and the later caller must
// not write the record twice or close a channel twice.
func (s *Supervisor) finish(id string, live *run, status string, exitCode *int, message string) bool {
	s.mu.Lock()
	if live.finalised {
		s.mu.Unlock()
		return false
	}
	live.finalised = true
	live.exited = true
	delete(s.runs, id)
	s.mu.Unlock()

	s.dropBaseline(live.pid)

	// Waiters observe the terminal state first: a caller released before the
	// record is written would still read "stopping" and a project restart
	// would skip the application it just stopped.
	s.update(id, status, exitCode, message, false)
	s.emitState(id, status, exitCode, message)
	close(live.done)
	return true
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
	deadline := time.Now().Add(stopTimeout)
	for {
		select {
		case <-live.done:
			return nil
		case <-time.After(stopPoll):
		}
		// The kill worked but its exit record has not arrived: a child that
		// outlived the tree can hold the output pipes open, and a loaded machine
		// can delay the supervisor. The process's own state is the fact that
		// decides, not the timeout alone.
		if pid > 0 && !processAlive(pid) {
			s.finish(id, live, StatusStopped, nil, "")
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("application process %d did not stop", pid)
		}
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
// pid, or returns empty when the process is not one of ours. The runs are
// copied under the lock so the process walk never holds it.
func (s *Supervisor) supervisedApplication(pid int) string {
	type candidate struct {
		pid  int
		name string
	}

	s.mu.Lock()
	candidates := make([]candidate, 0, len(s.runs))
	for _, live := range s.runs {
		if live.pid > 0 {
			candidates = append(candidates, candidate{pid: live.pid, name: live.name})
		}
	}
	s.mu.Unlock()

	for _, entry := range candidates {
		if ownedBy(pid, entry.pid) {
			return entry.name
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

// trackRuntime records the ports the tree listens on and whether the current
// port accepts connections, for as long as the process lives.
func (s *Supervisor) trackRuntime(id string, live *run, root int) {
	ticker := time.NewTicker(runtimeScanInterval)
	defer ticker.Stop()
	for {
		s.scanRuntime(id, live, root)
		select {
		case <-live.done:
			return
		case <-ticker.C:
		}
	}
}

func (s *Supervisor) scanRuntime(id string, live *run, root int) {
	if exited(live) {
		return
	}
	if listeners, err := listeningFor(); err == nil {
		s.recordPorts(id, live, portsOwnedBy(listeners, root))
	}
	s.recordHealth(id, live)
	s.recordUsage(id, live, root)
}

// exited reports whether the process is already gone. A scan that started just
// before the exit must not write a reading onto a record that has been cleared.
func exited(live *run) bool {
	select {
	case <-live.done:
		return true
	default:
		return false
	}
}

func (s *Supervisor) recordPorts(id string, live *run, ports []int) {
	if len(ports) == 0 {
		// Nothing observed yet: keep the last reading rather than flapping to
		// "no port" between a server's binds.
		return
	}

	s.mu.Lock()
	changed := !slices.Equal(live.ports, ports)
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

func (s *Supervisor) recordHealth(id string, live *run) {
	s.mu.Lock()
	port := 0
	if len(live.ports) > 0 {
		port = live.ports[0]
	}
	s.mu.Unlock()
	if port == 0 {
		// Not bound yet: unknown, not unhealthy.
		return
	}

	health := HealthHealthy
	detail := fmt.Sprintf("Answering on port %d.", port)
	if err := healthProbe(port); err != nil {
		health = HealthUnhealthy
		detail = fmt.Sprintf("No connection was accepted on port %d.", port)
	}
	if exited(live) {
		return
	}

	s.mu.Lock()
	changed := live.health != health || live.healthDetail != detail
	if changed {
		live.health = health
		live.healthDetail = detail
	}
	s.mu.Unlock()
	if !changed {
		return
	}

	if !s.updateHealth(id, health, detail) {
		return
	}
	if s.emit != nil {
		s.emit(EventHealth, map[string]any{
			"applicationId": id,
			"health":        health,
			"detail":        detail,
			"port":          port,
		})
	}
}

func (s *Supervisor) updatePort(id string, port int) {
	if s.database == nil {
		return
	}
	_, _ = s.database.Exec(`UPDATE applications SET port = ? WHERE id = ?`, port, id)
}

// updateHealth records a reading only while the record is still live. A probe
// that was in flight when the process ended must not leave a health reading on
// a stopped record; the status filter makes the stop's clearing the last word.
func (s *Supervisor) updateHealth(id string, health string, detail string) bool {
	if s.database == nil {
		return false
	}
	result, err := s.database.Exec(`UPDATE applications
		SET health = ?, health_detail = ?,
			health_checked_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		WHERE id = ? AND status IN (?, ?)`, health, detail, id, StatusStarting, StatusRunning)
	if err != nil {
		return false
	}
	affected, err := result.RowsAffected()
	return err == nil && affected > 0
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
				health = '', health_detail = '', health_checked_at = '',
				started_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
			WHERE id = ?`, status, id)
		return
	}
	_, _ = s.database.Exec(`UPDATE applications
		SET status = ?, exit_code = ?, error = ?,
			health = '', health_detail = '', health_checked_at = ''
		WHERE id = ?`, status, code, message, id)
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

// resolveExecutableIn resolves a bare program name in the assigned runtime's
// directory before the machine PATH, so `php artisan serve` runs the php the
// application pinned. A name that names a path is untouched.
func resolveExecutableIn(name string, directory string, runtimeDir string) (string, error) {
	if runtimeDir != "" && !strings.ContainsAny(name, `/\`) {
		if path := findInDir(runtimeDir, name); path != "" {
			return path, nil
		}
	}
	return resolveExecutable(name, directory)
}

// findInDir names the executable a directory holds for a bare program name,
// honouring Windows PATHEXT the way LookPath does.
func findInDir(dir string, name string) string {
	if runtime.GOOS == "windows" {
		extensions := pathExtensions()
		if filepath.Ext(name) != "" {
			extensions = []string{""}
		}
		for _, extension := range extensions {
			candidate := filepath.Join(dir, name+extension)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
		return ""
	}
	candidate := filepath.Join(dir, name)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
		return candidate
	}
	return ""
}

// pathExtensions lists the executable extensions a Windows PATH search
// honours, from PATHEXT or the system default.
func pathExtensions() []string {
	pathExt := os.Getenv("PATHEXT")
	if strings.TrimSpace(pathExt) == "" {
		return []string{".com", ".exe", ".bat", ".cmd"}
	}
	return strings.Split(strings.ToLower(pathExt), ";")
}

// prependPath puts a directory ahead of the PATH in env, so an assigned
// runtime resolves before the machine default for children too.
func prependPath(env []string, dir string) []string {
	for index, entry := range env {
		if len(entry) > 5 && strings.EqualFold(entry[:5], "PATH=") {
			env[index] = "PATH=" + dir + string(os.PathListSeparator) + entry[5:]
			return env
		}
		if len(entry) == 4 && strings.EqualFold(entry, "PATH") {
			env[index] = "PATH=" + dir
			return env
		}
	}
	return append(env, "PATH="+dir)
}
