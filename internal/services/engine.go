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

// postgresBinaries is a resolved engine: the three programs the lifecycle needs,
// all from one installation.
type postgresBinaries struct {
	postgres string
	initdb   string
	pgctl    string
	createdb string
	version  string
	location string
}

// findPostgres is a seam so tests describe an engine without one installed.
var findPostgres = discoverPostgres

// engineVersionTimeout bounds the version probe; an engine that cannot answer
// in this window is reported as unusable rather than blocking the catalogue.
const engineVersionTimeout = 10 * time.Second

// discoverPostgres finds the newest PostgreSQL installation whose programs this
// machine can run.
func discoverPostgres() (postgresBinaries, error) {
	directories := postgresDirectories()
	if len(directories) == 0 {
		return postgresBinaries{}, fmt.Errorf("no PostgreSQL installation was found in the usual locations")
	}

	type candidate struct {
		directory string
		major     int
	}
	candidates := make([]candidate, 0, len(directories))
	for _, directory := range directories {
		if !hasPrograms(directory) {
			continue
		}
		candidates = append(candidates, candidate{directory: directory, major: majorFromPath(directory)})
	}
	if len(candidates) == 0 {
		return postgresBinaries{}, fmt.Errorf("PostgreSQL was found but its programs are incomplete")
	}
	// Newest installation first, and stable for equal majors so a tie does not
	// depend on discovery order.
	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].major > candidates[right].major
	})

	binaries := postgresBinaries{location: candidates[0].directory}
	for _, entry := range candidates {
		binaries.postgres = filepath.Join(entry.directory, executable("postgres"))
		binaries.initdb = filepath.Join(entry.directory, executable("initdb"))
		binaries.pgctl = filepath.Join(entry.directory, executable("pg_ctl"))
		binaries.createdb = filepath.Join(entry.directory, executable("createdb"))
		version, err := postgresVersion(binaries.postgres)
		if err != nil {
			continue
		}
		binaries.version = version
		binaries.location = entry.directory
		return binaries, nil
	}
	return postgresBinaries{}, fmt.Errorf("PostgreSQL was found at %s but did not report its version", candidates[0].directory)
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

func hasPrograms(directory string) bool {
	for _, name := range []string{"postgres", "initdb", "pg_ctl", "createdb"} {
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

// postgresVersion asks the program for its version rather than trusting a
// folder name.
func postgresVersion(bin string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), engineVersionTimeout)
	defer cancel()
	command := process.CommandFor(ctx, bin, []string{"--version"})
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read PostgreSQL version: %w", err)
	}
	match := postgresVersionPattern.FindStringSubmatch(string(output))
	if match == nil {
		return "", fmt.Errorf("unexpected version output %q", strings.TrimSpace(string(output)))
	}
	return match[1], nil
}

// missingEngineDetail is the sentence the interface shows when the engine is not
// installed. It names what is missing instead of failing at start.
func missingEngineDetail(err error) string {
	return fmt.Sprintf("PostgreSQL is not installed on this machine, so a service cannot run yet. %s", err)
}
