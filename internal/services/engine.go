package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"locafy.dev/desktop/internal/process"
)

// postgresTemplate is the catalogue entry for PostgreSQL (ADR-0020 lists why
// engines arrive one at a time: each needs a verified native build per
// platform).
var postgresTemplate = Template{
	Engine:      EnginePostgreSQL,
	Name:        "PostgreSQL",
	Summary:     "Relational database, one data directory and port per project.",
	DefaultPort: 5432,
}

// mysqlTemplate is the catalogue entry for MySQL, the second engine ADR-0020
// admits: MySQL publishes vendor builds for Windows, macOS and Linux.
var mysqlTemplate = Template{
	Engine:      EngineMySQL,
	Name:        "MySQL",
	Summary:     "Relational database, one data directory and port per project.",
	DefaultPort: 3306,
}

// binaries is a resolved engine installation: the programs the lifecycle needs,
// all taken from one installation directory.
type binaries struct {
	engine   string
	server   string
	init     string
	control  string
	client   string
	version  string
	location string
}

// findEngine resolves the programs for one engine kind. It is a seam so a test
// describes an engine without one installed.
var findEngine = discoverEngine

// findPostgres and findMySQL are the per-engine discovery seams; tests replace
// them rather than the dispatch.
var (
	findPostgres = discoverPostgres
	findMySQL    = discoverMySQL
)

// engineVersionTimeout bounds the version probe; an engine that cannot answer
// in this window is reported as unusable rather than blocking the catalogue.
const engineVersionTimeout = 10 * time.Second

func discoverEngine(engine string) (binaries, error) {
	switch engine {
	case EnginePostgreSQL:
		return findPostgres()
	case EngineMySQL:
		return findMySQL()
	}
	return binaries{}, ErrUnknownEngine
}

// discoverPostgres finds the newest PostgreSQL installation whose programs this
// machine can run.
func discoverPostgres() (binaries, error) {
	directories := postgresDirectories()
	if len(directories) == 0 {
		return binaries{}, fmt.Errorf("no PostgreSQL installation was found in the usual locations")
	}

	type candidate struct {
		directory string
		major     int
	}
	candidates := make([]candidate, 0, len(directories))
	for _, directory := range directories {
		if !hasPrograms(directory, "postgres", "initdb", "pg_ctl", "createdb") {
			continue
		}
		candidates = append(candidates, candidate{directory: directory, major: majorFromPath(directory)})
	}
	if len(candidates) == 0 {
		return binaries{}, fmt.Errorf("PostgreSQL was found but its programs are incomplete")
	}
	// Newest installation first, and stable for equal majors so a tie does not
	// depend on discovery order.
	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].major > candidates[right].major
	})

	for _, entry := range candidates {
		programs := binaries{
			engine:   EnginePostgreSQL,
			server:   filepath.Join(entry.directory, executable("postgres")),
			init:     filepath.Join(entry.directory, executable("initdb")),
			control:  filepath.Join(entry.directory, executable("pg_ctl")),
			client:   filepath.Join(entry.directory, executable("createdb")),
			location: entry.directory,
		}
		version, err := postgresVersion(programs.server)
		if err != nil {
			continue
		}
		programs.version = version
		return programs, nil
	}
	return binaries{}, fmt.Errorf("PostgreSQL was found at %s but did not report its version", candidates[0].directory)
}

// discoverMySQL finds the newest MySQL installation whose programs this machine
// can run. MySQL ships a vendor build for every supported platform, which is
// what ADR-0020's admission rule asks for.
func discoverMySQL() (binaries, error) {
	directories := mysqlDirectories()
	if len(directories) == 0 {
		return binaries{}, fmt.Errorf("no MySQL installation was found in the usual locations")
	}

	type candidate struct {
		directory string
		major     int
	}
	candidates := make([]candidate, 0, len(directories))
	for _, directory := range directories {
		if !hasPrograms(directory, "mysqld", "mysql", "mysqladmin") {
			continue
		}
		candidates = append(candidates, candidate{directory: directory, major: majorFromPath(directory)})
	}
	if len(candidates) == 0 {
		return binaries{}, fmt.Errorf("MySQL was found but its programs are incomplete")
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].major > candidates[right].major
	})

	for _, entry := range candidates {
		programs := binaries{
			engine:   EngineMySQL,
			server:   filepath.Join(entry.directory, executable("mysqld")),
			init:     filepath.Join(entry.directory, executable("mysqld")),
			control:  filepath.Join(entry.directory, executable("mysqladmin")),
			client:   filepath.Join(entry.directory, executable("mysql")),
			location: entry.directory,
		}
		version, err := mysqlVersion(programs.server)
		if err != nil {
			continue
		}
		programs.version = version
		return programs, nil
	}
	return binaries{}, fmt.Errorf("MySQL was found at %s but did not report its version", candidates[0].directory)
}

// postgresDirectories returns the folders that may hold PostgreSQL programs:
// every copy on PATH, the platform's installation roots, and Locafy's own
// managed engines.
func postgresDirectories() []string {
	var directories []string
	if path, err := exec.LookPath(executable("postgres")); err == nil {
		directories = append(directories, filepath.Dir(path))
	}

	patterns := []string{}
	switch runtime.GOOS {
	case "windows":
		roots := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), `C:\`}
		for _, root := range roots {
			if root == "" {
				continue
			}
			patterns = append(patterns, filepath.Join(root, "PostgreSQL", "*", "bin"))
		}
		// Locafy's managed engines, unpacked by the install machinery.
		if root, err := DataRoot(); err == nil {
			patterns = append(patterns, filepath.Join(filepath.Dir(root), "engines", "postgresql", "*", "bin"))
		}
	default:
		patterns = append(patterns,
			"/usr/lib/postgresql/*/bin",
			"/usr/local/pgsql/bin",
			"/opt/homebrew/opt/postgresql@*/bin",
			"/usr/local/opt/postgresql@*/bin",
			"/opt/local/lib/postgresql*/bin",
			"/Applications/Postgres.app/Contents/Versions/*/bin",
		)
	}

	return expandDirectories(directories, patterns)
}

// mysqlDirectories returns the folders that may hold MySQL programs, including
// the common development-stack layouts (Laragon on Windows, Homebrew on macOS).
func mysqlDirectories() []string {
	var directories []string
	if path, err := exec.LookPath(executable("mysqld")); err == nil {
		directories = append(directories, filepath.Dir(path))
	}

	patterns := []string{}
	switch runtime.GOOS {
	case "windows":
		roots := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")}
		for _, root := range roots {
			if root == "" {
				continue
			}
			patterns = append(patterns,
				filepath.Join(root, "MySQL", "MySQL Server *", "bin"),
				filepath.Join(root, "MariaDB*", "bin"),
			)
		}
		patterns = append(patterns, `C:\laragon\bin\mysql\*\bin`)
		if root, err := DataRoot(); err == nil {
			patterns = append(patterns, filepath.Join(filepath.Dir(root), "engines", "mysql", "*", "bin"))
		}
	default:
		patterns = append(patterns,
			"/usr/local/mysql/bin",
			"/opt/homebrew/opt/mysql*/bin",
			"/usr/local/opt/mysql*/bin",
			"/Applications/MAMP/Library/bin",
		)
		if root, err := DataRoot(); err == nil {
			patterns = append(patterns, filepath.Join(filepath.Dir(root), "engines", "mysql", "*", "bin"))
		}
	}

	return expandDirectories(directories, patterns)
}

// expandDirectories glues the PATH answer to the platform patterns, dropping
// duplicates while keeping the order.
func expandDirectories(directories []string, patterns []string) []string {
	seen := map[string]bool{}
	for _, directory := range directories {
		seen[filepath.Clean(directory)] = true
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			clean := filepath.Clean(match)
			if seen[clean] {
				continue
			}
			seen[clean] = true
			directories = append(directories, clean)
		}
	}
	return directories
}

func hasPrograms(directory string, names ...string) bool {
	for _, name := range names {
		info, err := os.Stat(filepath.Join(directory, executable(name)))
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func executable(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// majorFromPath reads the version from the folder that holds the programs, such
// as .../PostgreSQL/17/bin, when the program itself has not answered yet.
func majorFromPath(directory string) int {
	parts := strings.Split(filepath.ToSlash(filepath.Dir(directory)), "/")
	for index := len(parts) - 1; index >= 0; index-- {
		digits := versionDigits.FindString(parts[index])
		if digits == "" {
			continue
		}
		if major, err := strconv.Atoi(strings.SplitN(digits, ".", 2)[0]); err == nil {
			return major
		}
	}
	return 0
}

var versionDigits = regexp.MustCompile(`\d+(?:\.\d+)*`)

var postgresVersionPattern = regexp.MustCompile(`\(PostgreSQL\)\s+([0-9][0-9.]*)`)

// mysqlVersionPattern reads the version from `mysqld --version`, whose output is
// "… Ver 8.0.30 for Win64 on x86_64 (MySQL Community Server - GPL)".
var mysqlVersionPattern = regexp.MustCompile(`Ver\s+([0-9][0-9.]*)`)

// postgresVersion asks the program for its version rather than trusting a
// folder name.
func postgresVersion(bin string) (string, error) {
	return probeVersion(bin, postgresVersionPattern)
}

func mysqlVersion(bin string) (string, error) {
	return probeVersion(bin, mysqlVersionPattern)
}

func probeVersion(bin string, pattern *regexp.Regexp) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), engineVersionTimeout)
	defer cancel()
	command := process.CommandFor(ctx, bin, []string{"--version"})
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read %s version: %w", filepath.Base(bin), err)
	}
	match := pattern.FindStringSubmatch(string(output))
	if match == nil {
		return "", fmt.Errorf("unexpected version output %q", strings.TrimSpace(string(output)))
	}
	return match[1], nil
}

// engineName is the catalogue name for an engine kind, used in messages.
func engineName(engine string) string {
	for _, template := range templates() {
		if template.Engine == engine {
			return template.Name
		}
	}
	return engine
}

// missingEngineDetail is the sentence the interface shows when the engine is not
// installed. It names what is missing instead of failing at start.
func missingEngineDetail(engine string, err error) string {
	return fmt.Sprintf("%s is not installed on this machine, so a service cannot run yet. %s", engineName(engine), err)
}
