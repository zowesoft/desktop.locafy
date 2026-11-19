// Package tools finds the executables installed on this machine: it searches
// PATH and the places installers put copies beside it, asks each copy for its
// version, and answers for consumers such as the framework scaffold and the
// runtime registry. Discovery reads the machine and stores nothing.
package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/process"
)

// Candidate is one installed copy of a tool, so an operator with several PHP
// or Node versions can pick the one a task should use.
type Candidate struct {
	Path    string
	Version string
	Source  string
}

var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// Versions are cached for the life of the process: probing a dozen candidate
// executables must not repeat on every preflight, and a tool does not change
// while Locafy is running.
var (
	versionCache   = map[string]string{}
	versionCacheMu sync.RWMutex
)

// hostOS is the platform whose install locations are searched, so the tests
// can cover every platform from one machine.
var hostOS = runtime.GOOS

// Find lists every installed copy of a tool, newest first. It is the
// discovery seam: tests replace it with a fixed set of candidates instead of
// probing the machine.
var Find = find

// find discovers one tool's copies: PATH first, then the places installers
// put copies beside it, then each copy's own version flag.
func find(name string) []Candidate {
	var candidates []Candidate
	seen := map[string]bool{}
	add := func(path string, source string) {
		path = strings.TrimSpace(path)
		if path == "" || !usableCopy(name, path) {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		// Homebrew and the system link several names at one binary, so a copy
		// reached twice must not appear twice in the picker.
		key := filepath.Clean(path)
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			key = filepath.Clean(resolved)
		}
		if seen[strings.ToLower(key)] {
			return
		}
		seen[strings.ToLower(key)] = true
		candidates = append(candidates, Candidate{Path: path, Source: source})
	}

	for _, search := range searchNames(name) {
		if path, err := exec.LookPath(search); err == nil {
			add(path, "PATH")
		}
	}

	// "python" on PATH is one copy among many; the launcher knows the rest.
	if name == "python" {
		for _, path := range pyLauncherPythons() {
			add(path, "installed")
		}
	}

	for _, path := range knownLocations(name) {
		matches, err := filepath.Glob(path)
		if err != nil {
			continue
		}
		for _, match := range matches {
			// A copy under Locafy's own runtimes directory was installed by an
			// install job, and the catalog marks it managed so the picker can
			// prefer it.
			source := "installed"
			if root := ManagedRoot(); root != "" && strings.HasPrefix(match, root) {
				source = "managed"
			}
			add(match, source)
		}
	}

	// Ask every candidate at once: on Windows each first launch can wait on an
	// antivirus scan, and the probe should not serialise a dozen of those.
	var wait sync.WaitGroup
	for index := range candidates {
		wait.Add(1)
		go func(candidate *Candidate) {
			defer wait.Done()
			candidate.Version = toolVersion(candidate.Path, name)
		}(&candidates[index])
	}
	wait.Wait()

	sort.SliceStable(candidates, func(i, j int) bool {
		return CompareVersions(candidates[i].Version, candidates[j].Version) > 0
	})
	return candidates
}

// pyLauncherPythons asks the Windows Python launcher which interpreters are
// registered. It lists Store installs and installers that never touched PATH,
// which a directory glob cannot see.
func pyLauncherPythons() []string {
	if hostOS != "windows" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := process.CommandFor(ctx, "py", []string{"-0p"})
	command.Env = append(os.Environ(), "NO_COLOR=1")
	output, err := command.CombinedOutput()
	if err != nil {
		return nil
	}
	return parsePyLauncher(string(output))
}

// parsePyLauncher reads the interpreter paths from `py -0p`, whose lines end
// with the executable they name.
func parsePyLauncher(output string) []string {
	var paths []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		index := strings.LastIndex(line, ":\\")
		if index < 1 {
			continue
		}
		// Paths may contain spaces (Program Files), so the executable suffix is
		// what separates an entry from the header line.
		path := strings.TrimSpace(line[index-1:])
		if !strings.HasSuffix(strings.ToLower(path), ".exe") {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

// searchNames are the executable names to ask PATH about. Unix names its
// interpreter python3, so a recipe that asks for python must still find a
// machine whose only copy is python3.
func searchNames(name string) []string {
	if name == "python" && hostOS != "windows" {
		return []string{"python", "python3"}
	}
	return []string{name}
}

// pythonBinary is the executable a Python location must name. A version glob
// also matches companions such as python3.12-config, which are not interpreters.
var pythonBinary = regexp.MustCompile(`(?i)^python(\d+(\.\d+)*)?(\.exe)?$`)

func usableCopy(name string, path string) bool {
	if name != "python" {
		return true
	}
	// The location tables name Windows executables even when the platform is
	// simulated, so the base is taken after either separator.
	base := path
	if index := strings.LastIndexAny(base, `/\`); index >= 0 {
		base = base[index+1:]
	}
	return pythonBinary.MatchString(base)
}

// toolVersion runs the tool's own version flag with a short timeout, twice —
// a first launch on Windows can stall on an antivirus scan, and two attempts
// catch the slow start without hanging discovery. An answer that never parses
// is not cached, so the next catalog read asks the copy again instead of
// calling it unknown for the whole session.
func toolVersion(path string, name string) string {
	versionCacheMu.RLock()
	cached, ok := versionCache[path]
	versionCacheMu.RUnlock()
	if ok {
		return cached
	}

	version := ""
	for attempt := 0; attempt < 2; attempt++ {
		version = probe(path, name)
		if version != "" {
			break
		}
	}
	if version != "" {
		versionCacheMu.Lock()
		versionCache[path] = version
		versionCacheMu.Unlock()
	}
	return version
}

// probe is the version-probe seam: tests replace it instead of launching
// executables.
var probe = probeOnce

// probeOnce runs the tool's own version flag once with a short timeout.
func probeOnce(path string, name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	args := []string{"--version"}
	if name == "php" {
		args = []string{"-v"}
	}
	// Go takes the version as a subcommand, not a flag; --version prints
	// usage, which parses to nothing or to the wrong number.
	if name == "go" {
		args = []string{"version"}
	}
	command := process.CommandFor(ctx, path, args)
	command.Env = append(os.Environ(), "NO_COLOR=1")
	output, _ := command.CombinedOutput()
	return parseVersion(string(output))
}

func parseVersion(output string) string {
	if match := versionPattern.FindString(strings.ReplaceAll(output, "\x1b", "")); match != "" {
		return match
	}
	return ""
}

// knownLocations lists the places a tool's installers put an executable beside
// PATH, as globs. Each platform only searches its own locations: a Windows
// path means nothing on macOS or Linux, and an unset environment variable must
// never turn into a relative path under the application's working directory.
func knownLocations(name string) []string {
	var locations []string
	switch hostOS {
	case "windows":
		locations = windowsLocations(name)
	case "darwin":
		locations = darwinLocations(name)
	default:
		locations = linuxLocations(name)
	}
	// A runtime Locafy installed itself lives in the managed runtimes
	// directory and is found like any other copy, but marked as managed.
	locations = append(locations, managedLocations(name)...)

	// An unset base directory contributes nothing rather than an empty glob.
	compacted := locations[:0]
	for _, location := range locations {
		if location != "" {
			compacted = append(compacted, location)
		}
	}
	return compacted
}

// ManagedRoot is the directory Locafy unpacks managed runtimes into: a
// platform data directory Locafy owns, named so discovery can find what the
// install jobs produce. Empty when the platform cannot say its data home.
func ManagedRoot() string {
	home, err := db.DataHome()
	if err != nil {
		return ""
	}
	return filepath.Join(home, db.AppDirName(), "runtimes")
}

// managedLocations lists the globs inside the managed runtimes directory,
// where an install job unpacks one version per folder.
func managedLocations(name string) []string {
	root := ManagedRoot()
	if root == "" {
		return nil
	}
	if hostOS == "windows" {
		return []string{locate(root, name, "*", name+".exe")}
	}
	return []string{
		locate(root, name, "*", name),
		locate(root, name, "*", "bin", name),
	}
}

func homeDirectory() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return os.Getenv("HOME")
}

// locate joins parts onto a base directory, returning nothing when the base is
// unknown so a missing environment variable cannot name a relative path.
func locate(base string, parts ...string) string {
	if base == "" {
		return ""
	}
	return filepath.Join(append([]string{base}, parts...)...)
}

// nodeVersionManagers are the per-user Node installs that keep every version
// under the home directory instead of PATH: nvm, Volta and fnm.
func nodeVersionManagers(name string) []string {
	home := homeDirectory()
	return []string{
		locate(home, ".nvm", "versions", "node", "*", "bin", name),
		locate(home, ".volta", "bin", name),
		locate(home, "Library", "Application Support", "fnm", "node-versions", "*", "installation", "bin", name),
		locate(home, ".local", "share", "fnm", "node-versions", "*", "installation", "bin", name),
	}
}

func windowsLocations(name string) []string {
	appData := os.Getenv("APPDATA")
	localAppData := os.Getenv("LOCALAPPDATA")
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	switch name {
	case "php":
		return []string{
			`C:\laragon\bin\php\php-*\php.exe`,
			`C:\xampp\php\php.exe`,
			`C:\tools\php*\php.exe`,
			`C:\php\php.exe`,
			locate(programFiles, "php*", "php.exe"),
		}
	case "composer":
		return []string{
			locate(appData, "Composer", "vendor", "bin", "composer.bat"),
			`C:\ProgramData\ComposerSetup\bin\composer.bat`,
			`C:\Composer\composer.bat`,
			`C:\laragon\bin\composer\composer.bat`,
		}
	case "laravel":
		return []string{
			locate(appData, "Composer", "vendor", "bin", "laravel.bat"),
			locate(appData, "Composer", "vendor", "bin", "laravel"),
			`C:\ProgramData\ComposerSetup\bin\laravel.bat`,
		}
	case "node", "npm", "npx":
		extension := ".exe"
		if name != "node" {
			extension = ".cmd"
		}
		return []string{
			locate(programFiles, "nodejs", name+extension),
			locate(programFilesX86, "nodejs", name+extension),
			locate(localAppData, "Programs", "nodejs", name+extension),
			`C:\nvm4w\nodejs\` + name + extension,
			locate(appData, "nvm", "v*", name+extension),
			locate(localAppData, "Volta", "bin", name+extension),
		}
	case "go":
		return []string{
			locate(programFiles, "Go", "bin", "go.exe"),
			locate(localAppData, "Programs", "Go", "bin", "go.exe"),
			`C:\Go\bin\go.exe`,
		}
	case "java":
		return []string{
			locate(programFiles, "Java", "jdk*", "bin", "java.exe"),
			locate(programFilesX86, "Java", "jdk*", "bin", "java.exe"),
			locate(programFiles, "Eclipse Adoptium", "jdk*", "bin", "java.exe"),
			locate(programFiles, "Microsoft", "jdk*", "bin", "java.exe"),
		}
	case "ruby":
		return []string{
			`C:\Ruby*\bin\ruby.exe`,
			locate(appData, "RubyInstaller", "ruby*", "bin", "ruby.exe"),
		}
	case "dotnet":
		return []string{
			locate(programFiles, "dotnet", "dotnet.exe"),
			locate(programFilesX86, "dotnet", "dotnet.exe"),
		}
	case "python":
		return []string{
			locate(localAppData, "Programs", "Python", "Python3*", "python.exe"),
			locate(programFiles, "Python3*", "python.exe"),
			`C:\Python3*\python.exe`,
			// Laragon ships its own interpreters, which may be older than a
			// separately installed Python.
			`C:\laragon\bin\python\python-*\python.exe`,
		}
	}
	return nil
}

func darwinLocations(name string) []string {
	home := homeDirectory()

	switch name {
	case "php":
		return []string{
			"/opt/homebrew/bin/php",
			"/usr/local/bin/php",
			"/usr/bin/php",
			"/Applications/MAMP/bin/php/php*/bin/php",
			locate(home, ".config", "herd", "bin", "php"),
		}
	case "composer":
		return []string{
			"/opt/homebrew/bin/composer",
			"/usr/local/bin/composer",
			locate(home, ".config", "composer", "vendor", "bin", "composer"),
			locate(home, ".composer", "vendor", "bin", "composer"),
		}
	case "laravel":
		return []string{
			"/opt/homebrew/bin/laravel",
			"/usr/local/bin/laravel",
			locate(home, ".config", "composer", "vendor", "bin", "laravel"),
			locate(home, ".composer", "vendor", "bin", "laravel"),
		}
	case "node", "npm", "npx":
		return append([]string{
			"/opt/homebrew/bin/" + name,
			"/usr/local/bin/" + name,
			"/usr/bin/" + name,
		}, nodeVersionManagers(name)...)
	case "go":
		return []string{
			"/usr/local/go/bin/go",
			"/opt/homebrew/bin/go",
			"/usr/local/bin/go",
		}
	case "java":
		return []string{
			"/usr/bin/java",
			locate(home, ".sdkman", "candidates", "java", "current", "bin", "java"),
		}
	case "ruby":
		return []string{
			"/usr/bin/ruby",
			"/usr/local/bin/ruby",
			"/opt/homebrew/bin/ruby",
			locate(home, ".rbenv", "versions", "*", "bin", "ruby"),
			locate(home, ".asdf", "installs", "ruby", "*", "bin", "ruby"),
		}
	case "dotnet":
		return []string{
			"/usr/local/share/dotnet/dotnet",
			"/opt/homebrew/bin/dotnet",
			"/usr/local/bin/dotnet",
			locate(home, ".dotnet", "dotnet"),
		}
	case "python":
		return []string{
			"/opt/homebrew/bin/python3",
			"/opt/homebrew/bin/python3.*",
			"/usr/local/bin/python3",
			"/usr/local/bin/python3.*",
			"/usr/bin/python3",
			locate(home, ".pyenv", "versions", "*", "bin", "python3"),
			locate(home, ".asdf", "installs", "python", "*", "bin", "python3"),
			locate(home, ".local", "bin", "python3"),
		}
	}
	return nil
}

func linuxLocations(name string) []string {
	home := homeDirectory()

	switch name {
	case "php":
		return []string{
			"/usr/bin/php",
			"/usr/bin/php8.*",
			"/usr/local/bin/php",
			"/snap/bin/php",
			"/home/linuxbrew/.linuxbrew/bin/php",
		}
	case "composer":
		return []string{
			"/usr/bin/composer",
			"/usr/local/bin/composer",
			locate(home, ".config", "composer", "vendor", "bin", "composer"),
			locate(home, ".composer", "vendor", "bin", "composer"),
		}
	case "laravel":
		return []string{
			"/usr/local/bin/laravel",
			locate(home, ".config", "composer", "vendor", "bin", "laravel"),
			locate(home, ".composer", "vendor", "bin", "laravel"),
		}
	case "node", "npm", "npx":
		return append([]string{
			"/usr/bin/" + name,
			"/usr/local/bin/" + name,
			"/snap/bin/" + name,
			"/home/linuxbrew/.linuxbrew/bin/" + name,
		}, nodeVersionManagers(name)...)
	case "go":
		return []string{
			"/usr/local/go/bin/go",
			"/usr/bin/go",
			"/usr/local/bin/go",
			"/snap/bin/go",
			"/home/linuxbrew/.linuxbrew/bin/go",
		}
	case "java":
		return []string{
			"/usr/bin/java",
			locate(home, ".sdkman", "candidates", "java", "current", "bin", "java"),
		}
	case "ruby":
		return []string{
			"/usr/bin/ruby",
			"/usr/local/bin/ruby",
			"/snap/bin/ruby",
			"/home/linuxbrew/.linuxbrew/bin/ruby",
			locate(home, ".rbenv", "versions", "*", "bin", "ruby"),
			locate(home, ".asdf", "installs", "ruby", "*", "bin", "ruby"),
		}
	case "dotnet":
		return []string{
			"/usr/bin/dotnet",
			"/usr/lib/dotnet/dotnet",
			"/usr/share/dotnet/dotnet",
			"/usr/local/share/dotnet/dotnet",
			"/snap/bin/dotnet",
			locate(home, ".dotnet", "dotnet"),
		}
	case "python":
		return []string{
			"/usr/bin/python3",
			"/usr/bin/python3.*",
			"/usr/local/bin/python3",
			"/usr/local/bin/python3.*",
			"/snap/bin/python3",
			"/home/linuxbrew/.linuxbrew/bin/python3",
			locate(home, ".pyenv", "versions", "*", "bin", "python3"),
			locate(home, ".asdf", "installs", "python", "*", "bin", "python3"),
			locate(home, ".local", "bin", "python3"),
		}
	}
	return nil
}

// CompareVersions compares dotted versions, missing parts counting as zero.
func CompareVersions(a string, b string) int {
	left := versionNumbers(a)
	right := versionNumbers(b)
	for index := 0; index < 3; index++ {
		if left[index] != right[index] {
			if left[index] < right[index] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func versionNumbers(value string) [3]int {
	var numbers [3]int
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(value), "v"), ".")
	for index := 0; index < 3 && index < len(parts); index++ {
		digits := strings.Builder{}
		for _, r := range parts[index] {
			if r < '0' || r > '9' {
				break
			}
			digits.WriteRune(r)
		}
		numbers[index], _ = strconv.Atoi(digits.String())
	}
	return numbers
}

// AtLeast reports whether version is at least minimum.
func AtLeast(version string, minimum string) bool {
	return CompareVersions(version, minimum) >= 0
}

// HostOS names the platform whose install locations discovery searches, so
// consumers that suggest platform-specific fixes can ask without guessing.
func HostOS() string {
	return hostOS
}
