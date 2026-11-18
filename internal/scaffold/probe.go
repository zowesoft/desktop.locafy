package scaffold

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ToolCandidate is one installed copy of a tool, so an operator with several
// PHP or Node versions can pick the one the scaffold should use.
type ToolCandidate struct {
	Path    string
	Version string
	Source  string
}

// ToolStatus reports one tool a recipe needs, with every installed version
// found and the one that would be used.
type ToolStatus struct {
	Tool       string
	Label      string
	MinVersion string
	Required   bool
	Candidates []ToolCandidate
	Selected   string
	Version    string
	OK         bool
	Problem    string
	// Fix explains how to satisfy a required tool that is missing or too old.
	Fix *ToolFix
}

// Preflight is the answer to "can this recipe run here, and with what".
type Preflight struct {
	RecipeID  string
	Ready     bool
	Tools     []ToolStatus
	Commands  []string
	Installer bool
	Notes     []string
}

var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// Versions are cached for the life of the process: probing a dozen candidate
// executables must not repeat on every preflight, and a tool does not change
// while Locafy is running.
var (
	versionCache   = map[string]string{}
	versionCacheMu sync.RWMutex
)

// Probe finds the tools a recipe references. options are the resolved recipe
// choices, which decide a conditional requirement; tools maps a tool to the
// path the operator chose, where an empty or unknown selection falls back to
// the newest installed version that meets the minimum.
func Probe(recipe Recipe, options map[string]string, tools map[string]string) ([]ToolStatus, map[string]bool, map[string]string) {
	required := map[string]Requirement{}
	for _, requirement := range recipe.Requires {
		if !conditionAllows(requirement.When, recipe, options) {
			continue
		}
		required[requirement.Tool] = requirement
	}

	names := map[string]bool{}
	for name := range required {
		names[name] = true
	}
	for _, command := range recipe.Commands {
		name := strings.Trim(command.Bin, "{}")
		if name == "" || name == "venv_python" {
			continue
		}
		names[name] = true
	}
	for _, requirement := range recipe.Requires {
		names[requirement.Tool] = true
	}

	sortedNames := make([]string, 0, len(names))
	for name := range names {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	present := map[string]bool{}
	paths := map[string]string{}
	statuses := make([]ToolStatus, 0, len(sortedNames))

	for _, name := range sortedNames {
		requirement, isRequired := required[name]
		candidates := findToolFor(name)
		status := ToolStatus{
			Tool:       name,
			Label:      name,
			Required:   isRequired,
			Candidates: candidates,
		}
		if isRequired {
			status.Label = requirement.Label
			status.MinVersion = requirement.MinVersion
		}
		if name == "laravel" {
			// The interface appends "(optional)" for tools that are not required.
			status.Label = "Laravel installer"
		}
		if name == "npx" {
			status.Label = "npx"
		}

		present[name] = len(candidates) > 0
		chosen := chooseCandidate(candidates, tools[name], status.MinVersion)
		if chosen != nil {
			status.Selected = chosen.Path
			status.Version = chosen.Version
			paths[name] = chosen.Path
		}

		switch {
		case len(candidates) == 0 && isRequired:
			status.Problem = "not installed"
			status.Fix = fixFor(name, status.Label, status.MinVersion, "")
		case chosen != nil && isRequired && !AtLeast(chosen.Version, status.MinVersion):
			status.Problem = "needs " + status.MinVersion + " or newer"
			status.Fix = fixFor(name, status.Label, status.MinVersion, chosen.Version)
		default:
			status.OK = true
		}
		if isRequired && status.Problem == "" {
			status.OK = true
		}
		statuses = append(statuses, status)
	}
	return statuses, present, paths
}

// conditionAllows evaluates a requirement's condition against the chosen
// options, so a newer Django can require a newer Python.
func conditionAllows(expression string, recipe Recipe, selected map[string]string) bool {
	if strings.TrimSpace(expression) == "" {
		return true
	}
	condition, err := parseCondition(expression)
	if err != nil {
		return false
	}
	tools := map[string]bool{}
	for _, command := range recipe.Commands {
		tools[strings.Trim(command.Bin, "{}")] = true
	}
	return condition.matches(tools, selected)
}

func chooseCandidate(candidates []ToolCandidate, selected string, minimum string) *ToolCandidate {
	if selected != "" {
		for index := range candidates {
			if strings.EqualFold(candidates[index].Path, selected) {
				return &candidates[index]
			}
		}
	}
	var best *ToolCandidate
	for index := range candidates {
		candidate := &candidates[index]
		if minimum != "" && !AtLeast(candidate.Version, minimum) {
			continue
		}
		if best == nil || betterCandidate(candidate, best, minimum) {
			best = candidate
		}
	}
	if best != nil {
		return best
	}
	// Nothing meets the minimum: name the newest copy anyway, so the caller
	// can say which interpreter is too old.
	var fallback *ToolCandidate
	for index := range candidates {
		candidate := &candidates[index]
		if fallback == nil || compareVersions(candidate.Version, fallback.Version) > 0 {
			fallback = candidate
		}
	}
	return fallback
}

// betterCandidate prefers the newest usable version for required tools. For an
// optional tool such as the Laravel installer, where every copy is the same
// program, it prefers the canonical Composer install over an environment's
// wrapper (Herd, for example) so the operator gets the tool they chose, not
// whichever shim happens to be first on PATH.
func betterCandidate(candidate *ToolCandidate, best *ToolCandidate, minimum string) bool {
	if minimum == "" {
		if candidateRank(candidate) != candidateRank(best) {
			return candidateRank(candidate) < candidateRank(best)
		}
		return compareVersions(candidate.Version, best.Version) > 0
	}
	if comparison := compareVersions(candidate.Version, best.Version); comparison != 0 {
		return comparison > 0
	}
	return candidateRank(candidate) < candidateRank(best)
}

// candidateRank orders install locations from most to least canonical.
func candidateRank(candidate *ToolCandidate) int {
	path := strings.ToLower(filepath.ToSlash(candidate.Path))
	switch {
	case strings.Contains(path, "composersetup"), strings.Contains(path, "composer/vendor/bin"):
		return 0
	case strings.Contains(path, ".config/herd"), strings.Contains(path, "/herd/bin"):
		return 5
	case candidate.Source == "PATH":
		return 2
	default:
		return 1
	}
}

// findToolFor is the discovery seam: tests replace it with a fixed set of
// candidates instead of probing the machine.
var findToolFor = findTool

// pyLauncherPythons asks the Windows Python launcher which interpreters are
// registered. It lists Store installs and installers that never touched PATH,
// which a directory glob cannot see.
func pyLauncherPythons() []string {
	if hostOS != "windows" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := commandFor(ctx, "py", []string{"-0p"})
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
	return pythonBinary.MatchString(filepath.Base(path))
}

func findTool(name string) []ToolCandidate {
	var candidates []ToolCandidate
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
		candidates = append(candidates, ToolCandidate{Path: path, Source: source})
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
			add(match, "installed")
		}
	}

	// Ask every candidate at once: on Windows each first launch can wait on an
	// antivirus scan, and the probe should not serialise a dozen of those.
	var wait sync.WaitGroup
	for index := range candidates {
		wait.Add(1)
		go func(candidate *ToolCandidate) {
			defer wait.Done()
			candidate.Version = toolVersion(candidate.Path, name)
		}(&candidates[index])
	}
	wait.Wait()

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareVersions(candidates[i].Version, candidates[j].Version) > 0
	})
	return candidates
}

// toolVersion runs the tool's own version flag with a short timeout. A tool
// that answers with something unparseable is kept, with an empty version.
func toolVersion(path string, name string) string {
	versionCacheMu.RLock()
	cached, ok := versionCache[path]
	versionCacheMu.RUnlock()
	if ok {
		return cached
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	args := []string{"--version"}
	if name == "php" {
		args = []string{"-v"}
	}
	command := commandFor(ctx, path, args)
	command.Env = append(os.Environ(), "NO_COLOR=1")
	output, _ := command.CombinedOutput()
	version := parseVersion(string(output))

	// A tool that answered with something unparseable is still a tool; cache
	// the empty answer so a slow executable is not launched again.
	versionCacheMu.Lock()
	versionCache[path] = version
	versionCacheMu.Unlock()
	return version
}

func parseVersion(output string) string {
	if match := versionPattern.FindString(strings.ReplaceAll(output, "\x1b", "")); match != "" {
		return match
	}
	return ""
}
