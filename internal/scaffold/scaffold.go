// Package scaffold turns a shipped recipe catalogue into new applications:
// choosing a framework shows its version options, checks the tools it needs and
// runs the framework's own installer with an argv it can print beforehand.
//
// Nothing here is fetched at run time. The catalogue is embedded in the binary
// and validated on load, so a recipe cannot ask for a shell, an unknown
// placeholder or a version that was never offered.
package scaffold

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed recipes.json
var recipeFS embed.FS

// Recipe is one framework Locafy can create an application from.
type Recipe struct {
	ID          string
	Label       string
	Language    string
	Framework   string
	Description string
	Docs        string
	VerifiedAt  string
	Options     []Option
	Requires    []Requirement
	Commands    []Command
	Verify      []string
	Application ApplicationSpec
}

// Option is a closed choice on a recipe. The values are the only strings a
// command can be bound to, which is why options are not free text.
type Option struct {
	Key     string
	Label   string
	Default string
	Values  []OptionValue
}

// OptionValue is one allowed choice, with the label the interface shows.
type OptionValue struct {
	Value string
	Label string
}

// Requirement names a tool the recipe needs, and the earliest version that
// works. Tools are detected, never installed by the scaffold.
type Requirement struct {
	Tool       string
	Label      string
	MinVersion string
	When       string
	Optional   bool
}

// ApplicationSpec is what the resulting application record starts with.
type ApplicationSpec struct {
	StartCommand string
	StopCommand  string
}

// Command is one step of a recipe. Cwd is "parent" (the folder holding the
// target, so the installer creates the target itself) or "inside" (the target,
// after Locafy created it).
type Command struct {
	Bin  string
	Cwd  string
	Args []Arg
	When string
}

// Arg is a fixed argument or a group that only applies when its condition
// holds, so one recipe can describe a starter-kit flag without a shell.
type Arg struct {
	Literal string
	When    string
	Group   []Arg
}

// UnmarshalJSON accepts either a string or a conditional group.
func (a *Arg) UnmarshalJSON(data []byte) error {
	var literal string
	if err := json.Unmarshal(data, &literal); err == nil {
		a.Literal = literal
		return nil
	}
	var group struct {
		When string `json:"when"`
		Args []Arg  `json:"args"`
	}
	if err := json.Unmarshal(data, &group); err != nil {
		return fmt.Errorf("argument must be a string or {when, args}: %w", err)
	}
	if strings.TrimSpace(group.When) == "" || len(group.Args) == 0 {
		return errors.New("a conditional argument needs a when and at least one argument")
	}
	a.When = group.When
	a.Group = group.Args
	return nil
}

// Catalogue is the validated set of recipes shipped with this build.
type Catalogue struct {
	Recipes []Recipe
}

var (
	// ErrRecipeNotFound is returned for an unknown recipe id.
	ErrRecipeNotFound = errors.New("framework recipe not found")
	// ErrOptionInvalid is returned when a requested option value was not offered.
	ErrOptionInvalid = errors.New("option value is not offered by the recipe")
	// ErrDestinationInvalid is returned when a target cannot be scaffolded into.
	ErrDestinationInvalid = errors.New("the destination is not usable")
	// ErrToolMissing is returned when a required tool is not installed.
	ErrToolMissing = errors.New("a required tool is not installed")
	// ErrTargetNotEmpty is returned rather than overwriting existing files.
	ErrTargetNotEmpty = errors.New("the destination folder is not empty")
)

var (
	loadOnce     sync.Once
	loaded       *Catalogue
	loadErr      error
	knownTokens  = map[string]bool{"dir": true, "name": true, "venv_python": true, "python": true}
	forbiddenArg = "<>|&;\n\r`$"
)

// Load returns the embedded catalogue, validating it once. A broken build
// fails here rather than halfway through an install.
func Load() (*Catalogue, error) {
	loadOnce.Do(func() {
		data, err := recipeFS.ReadFile("recipes.json")
		if err != nil {
			loadErr = fmt.Errorf("read recipes: %w", err)
			return
		}
		var document struct {
			Recipes []Recipe `json:"recipes"`
		}
		if err := json.Unmarshal(data, &document); err != nil {
			loadErr = fmt.Errorf("parse recipes: %w", err)
			return
		}
		catalogue := &Catalogue{Recipes: document.Recipes}
		if err := catalogue.validate(); err != nil {
			loadErr = err
			return
		}
		loaded = catalogue
	})
	return loaded, loadErr
}

// Find returns one recipe by id.
func (c *Catalogue) Find(id string) (Recipe, error) {
	for _, recipe := range c.Recipes {
		if recipe.ID == id {
			return recipe, nil
		}
	}
	return Recipe{}, ErrRecipeNotFound
}

func (c *Catalogue) validate() error {
	seen := map[string]bool{}
	for index := range c.Recipes {
		recipe := &c.Recipes[index]
		if strings.TrimSpace(recipe.ID) == "" || strings.TrimSpace(recipe.Label) == "" {
			return fmt.Errorf("recipe %d needs an id and a label", index)
		}
		if seen[recipe.ID] {
			return fmt.Errorf("recipe id %q is listed twice", recipe.ID)
		}
		seen[recipe.ID] = true
		if len(recipe.Commands) == 0 {
			return fmt.Errorf("recipe %q has no command", recipe.ID)
		}
		if len(recipe.Verify) == 0 {
			return fmt.Errorf("recipe %q declares nothing to verify", recipe.ID)
		}
		if recipe.Application.StartCommand == "" {
			return fmt.Errorf("recipe %q has no start command", recipe.ID)
		}

		options := map[string]bool{}
		for _, option := range recipe.Options {
			if option.Key == "" || len(option.Values) == 0 {
				return fmt.Errorf("recipe %q has an option without values", recipe.ID)
			}
			options[option.Key] = true
			offered := false
			for _, value := range option.Values {
				if value.Value == option.Default {
					offered = true
				}
				if strings.ContainsAny(value.Value, " \t"+forbiddenArg) {
					return fmt.Errorf("recipe %q option %q has an unsafe value %q", recipe.ID, option.Key, value.Value)
				}
			}
			if !offered {
				return fmt.Errorf("recipe %q default %q is not one of its values", recipe.ID, option.Default)
			}
			if _, err := expandTokens("option:"+option.Key, map[string]string{option.Key: option.Default}, nil); err != nil {
				return fmt.Errorf("recipe %q option %q: %w", recipe.ID, option.Key, err)
			}
		}

		for _, requirement := range recipe.Requires {
			if requirement.Tool == "" || requirement.MinVersion == "" {
				return fmt.Errorf("recipe %q has an incomplete requirement", recipe.ID)
			}
		}

		for _, command := range recipe.Commands {
			if strings.ContainsAny(command.Bin, `/\`) || command.Bin == "" {
				return fmt.Errorf("recipe %q command %q must name a tool, not a path", recipe.ID, command.Bin)
			}
			if command.Cwd != "parent" && command.Cwd != "inside" {
				return fmt.Errorf("recipe %q command %q has cwd %q", recipe.ID, command.Bin, command.Cwd)
			}
			if err := validateArgs(recipe.ID, command.Args, options); err != nil {
				return err
			}
			if command.When != "" {
				if _, err := parseCondition(command.When); err != nil {
					return fmt.Errorf("recipe %q command condition: %w", recipe.ID, err)
				}
			}
		}
	}
	return nil
}

func validateArgs(recipeID string, args []Arg, options map[string]bool) error {
	for _, arg := range args {
		if arg.When != "" {
			if _, err := parseCondition(arg.When); err != nil {
				return fmt.Errorf("recipe %q argument condition: %w", recipeID, err)
			}
			if err := validateArgs(recipeID, arg.Group, options); err != nil {
				return err
			}
			continue
		}
		if strings.ContainsAny(arg.Literal, forbiddenArg) {
			return fmt.Errorf("recipe %q argument %q contains a shell character", recipeID, arg.Literal)
		}
		for _, token := range tokens(arg.Literal) {
			if knownTokens[token] {
				continue
			}
			if !options[token] && token != "version" {
				return fmt.Errorf("recipe %q argument %q uses unknown token %q", recipeID, arg.Literal, token)
			}
		}
	}
	return nil
}

// Condition is a conjunction or disjunction of simple checks, enough for "use
// the installer when it is present" and "use the version-pinned command".
type Condition struct {
	all [][]clause
}

type clause struct {
	negate bool
	tool   string
	option string
	value  string
}

// parseCondition understands "a && b", "a || b" and "tool:missing:x".
func parseCondition(expression string) (Condition, error) {
	var condition Condition
	for _, alternative := range strings.Split(expression, "||") {
		var group []clause
		for _, term := range strings.Split(alternative, "&&") {
			term = strings.TrimSpace(term)
			switch {
			case strings.HasPrefix(term, "tool:missing:"):
				group = append(group, clause{negate: true, tool: strings.TrimPrefix(term, "tool:missing:")})
			case strings.HasPrefix(term, "tool:"):
				group = append(group, clause{tool: strings.TrimPrefix(term, "tool:")})
			case strings.HasPrefix(term, "option:") && strings.Contains(term, "="):
				parts := strings.SplitN(strings.TrimPrefix(term, "option:"), "=", 2)
				group = append(group, clause{option: parts[0], value: parts[1]})
			default:
				return Condition{}, fmt.Errorf("cannot read condition %q", term)
			}
		}
		condition.all = append(condition.all, group)
	}
	return condition, nil
}

// matches reports whether any alternative holds.
func (c Condition) matches(tools map[string]bool, options map[string]string) bool {
	if len(c.all) == 0 {
		return true
	}
	for _, group := range c.all {
		ok := true
		for _, term := range group {
			switch {
			case term.tool != "" && term.negate:
				ok = ok && !tools[term.tool]
			case term.tool != "":
				ok = ok && tools[term.tool]
			default:
				ok = ok && options[term.option] == term.value
			}
			if !ok {
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// ResolveOptions fills defaults and rejects values the recipe never offered.
func ResolveOptions(recipe Recipe, requested map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(recipe.Options))
	for _, option := range recipe.Options {
		value := strings.TrimSpace(requested[option.Key])
		if value == "" {
			value = option.Default
		}
		offered := false
		for _, candidate := range option.Values {
			if candidate.Value == value {
				offered = true
				break
			}
		}
		if !offered {
			return nil, fmt.Errorf("%w: %s=%s", ErrOptionInvalid, option.Key, value)
		}
		resolved[option.Key] = value
	}
	for key := range requested {
		found := false
		for _, option := range recipe.Options {
			if option.Key == key {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: no option %q", ErrOptionInvalid, key)
		}
	}
	return resolved, nil
}

// Step is one command with every token already bound. It is printed in the
// interface before anything runs.
type Step struct {
	Bin     string
	Args    []string
	Cwd     string
	Display string
}

// BuildSteps expands a recipe for the chosen options and detected tools.
// Chosen tool paths are used in place of the bare tool name, so the operator's
// version choice is honoured for the command and its child processes.
func BuildSteps(recipe Recipe, options map[string]string, tools map[string]bool, toolPaths map[string]string) ([]Step, error) {
	var steps []Step
	for _, command := range recipe.Commands {
		if command.When != "" {
			condition, err := parseCondition(command.When)
			if err != nil {
				return nil, err
			}
			if !condition.matches(tools, options) {
				continue
			}
		}
		args, err := expandArgs(command.Args, tools, options)
		if err != nil {
			return nil, err
		}
		tokenValues := optionTokens(options)
		tokenValues["dir"] = "{dir}"
		tokenValues["name"] = "{name}"
		tokenValues["python"] = "{python}"
		tokenValues["venv_python"] = "{venv_python}"

		bin := command.Bin
		if strings.HasPrefix(bin, "{") {
			expanded, err := expandTokens(bin, options, tokenValues)
			if err != nil {
				return nil, err
			}
			bin = expanded
		}
		if path := toolPaths[bin]; path != "" {
			bin = path
		}
		display := append([]string{bin}, args...)
		steps = append(steps, Step{
			Bin:     bin,
			Args:    args,
			Cwd:     command.Cwd,
			Display: strings.Join(display, " "),
		})
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("%w: no command matches the chosen options", ErrRecipeNotFound)
	}
	return steps, nil
}

func expandArgs(args []Arg, tools map[string]bool, options map[string]string) ([]string, error) {
	var expanded []string
	for _, arg := range args {
		if arg.When != "" {
			condition, err := parseCondition(arg.When)
			if err != nil {
				return nil, err
			}
			if !condition.matches(tools, options) {
				continue
			}
			nested, err := expandArgs(arg.Group, tools, options)
			if err != nil {
				return nil, err
			}
			expanded = append(expanded, nested...)
			continue
		}
		value, err := expandTokens(arg.Literal, options, optionTokens(options))
		if err != nil {
			return nil, err
		}
		expanded = append(expanded, value)
	}
	return expanded, nil
}

// expandTokens replaces {name} placeholders. Only catalogue-declared tokens
// are accepted; anything else is a build error, never a runtime surprise.
func expandTokens(value string, options map[string]string, extra map[string]string) (string, error) {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '{' {
			builder.WriteByte(value[index])
			continue
		}
		end := strings.IndexByte(value[index:], '}')
		if end < 0 {
			return "", fmt.Errorf("unclosed token in %q", value)
		}
		token := value[index+1 : index+end]
		replacement, ok := extra[token]
		if !ok {
			replacement, ok = options[token]
		}
		if !ok {
			if !knownTokens[token] {
				return "", fmt.Errorf("unknown token %q", token)
			}
			replacement = "{" + token + "}"
		}
		builder.WriteString(replacement)
		index += end
	}
	return builder.String(), nil
}

func optionTokens(options map[string]string) map[string]string {
	tokens := make(map[string]string, len(options))
	for key, value := range options {
		tokens[key] = value
	}
	return tokens
}

func tokens(value string) []string {
	var found []string
	for index := 0; index < len(value); index++ {
		if value[index] != '{' {
			continue
		}
		end := strings.IndexByte(value[index:], '}')
		if end < 0 {
			return found
		}
		found = append(found, value[index+1:index+end])
		index += end
	}
	return found
}

// ResolveDirectory validates a relative target below root and returns the
// cleaned absolute path and its slash-separated relative form.
func ResolveDirectory(root string, directory string) (string, string, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "", "", fmt.Errorf("%w: a folder is required", ErrDestinationInvalid)
	}
	absolute := directory
	if !filepath.IsAbs(directory) {
		absolute = filepath.Join(root, filepath.FromSlash(directory))
	}
	absolute = filepath.Clean(absolute)
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("%w: %q is outside the project", ErrDestinationInvalid, directory)
	}
	if relative == "." {
		return "", "", fmt.Errorf("%w: choose a folder inside the project", ErrDestinationInvalid)
	}
	return filepath.ToSlash(relative), absolute, nil
}

// TargetState describes a destination before anything runs.
type TargetState struct {
	Exists bool
	Empty  bool
}

// InspectTarget reports whether a destination already holds files.
func InspectTarget(path string) (TargetState, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		return TargetState{}, nil
	}
	if err != nil {
		return TargetState{}, fmt.Errorf("inspect %q: %w", path, err)
	}
	return TargetState{Exists: true, Empty: len(entries) == 0}, nil
}

// VenvPython is the interpreter a virtual environment exposes on this platform.
func VenvPython(target string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(target, ".venv", "Scripts", "python.exe")
	}
	return filepath.Join(target, ".venv", "bin", "python")
}

// Slugify makes a portable folder name from a label.
func Slugify(value string) string {
	var builder strings.Builder
	dash := false
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			dash = false
		case !dash && builder.Len() > 0:
			builder.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

// Identifier turns a name into something a project generator accepts.
func Identifier(value string) string {
	identifier := strings.ReplaceAll(Slugify(value), "-", "_")
	if identifier == "" {
		identifier = "app"
	}
	if identifier[0] >= '0' && identifier[0] <= '9' {
		identifier = "app_" + identifier
	}
	return identifier
}

// DirectoriesReady reports whether every verify marker exists under target.
func DirectoriesReady(target string, markers []string) ([]string, bool) {
	var missing []string
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(marker))); err != nil {
			missing = append(missing, marker)
		}
	}
	return missing, len(missing) == 0
}

// SortedVersions orders version strings from oldest to newest.
func SortedVersions(versions []string) []string {
	sorted := append([]string(nil), versions...)
	sort.Slice(sorted, func(i, j int) bool { return compareVersions(sorted[i], sorted[j]) < 0 })
	return sorted
}

// compareVersions compares dotted versions, missing parts counting as zero.
func compareVersions(a string, b string) int {
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
	return compareVersions(version, minimum) >= 0
}
