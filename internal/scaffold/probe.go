package scaffold

import (
	"path/filepath"
	"sort"
	"strings"

	"locafy.dev/desktop/internal/tools"
)

// ToolCandidate is one installed copy of a tool, so an operator with several
// PHP or Node versions can pick the one the scaffold should use.
type ToolCandidate = tools.Candidate

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
// candidates instead of probing the machine. Discovery itself lives in the
// tools package, which several consumers share.
var findToolFor = tools.Find

// FindCandidates returns every installed copy of a tool the registry names,
// newest first, with the version each copy's own flag reports and where it was
// found. It is the discovery entry point for consumers outside the scaffold
// runner, such as the runtime registry.
func FindCandidates(name string) []ToolCandidate {
	return findToolFor(name)
}

// pyLauncherPythons asks the Windows Python launcher which interpreters are
// registered. It lists Store installs and installers that never touched PATH,
// which a directory glob cannot see.
