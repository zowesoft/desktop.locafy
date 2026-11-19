package scaffold

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"locafy.dev/desktop/internal/applications"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/process"
	"locafy.dev/desktop/internal/projects"
	"locafy.dev/desktop/internal/tools"
)

// Service exposes the recipe catalogue and scaffold runs over the binding
// layer. It is the only part that knows about projects and applications; the
// runner deals in targets and argv.
type Service struct {
	database  *sql.DB
	catalogue *Catalogue
	manager   *Manager
}

// NewService returns a scaffold service backed by database, writing job logs
// under the application's log directory.
func NewService(database *sql.DB) *Service {
	catalogue, err := Load()
	if err != nil {
		// A broken catalogue is a build error; keep the service usable so the
		// application can report it instead of failing to start.
		catalogue = &Catalogue{}
	}

	logDir := ""
	if home, err := db.DataHome(); err == nil {
		logDir = filepath.Join(home, db.AppDirName(), "logs")
	}

	service := &Service{database: database, catalogue: catalogue}
	service.manager = NewManager(database, logDir, func(name string, data any) {
		if app := application.Get(); app != nil {
			app.Event.Emit(name, data)
		}
	})
	return service
}

// Recipes returns the shipped catalogue with its version options.
func (s *Service) Recipes() []Recipe {
	return s.catalogue.Recipes
}

// Preflight reports the tools this recipe needs, every installed version of
// them and the exact commands that would run.
func (s *Service) Preflight(recipeID string, options map[string]string) (Preflight, error) {
	recipe, err := s.catalogue.Find(recipeID)
	if err != nil {
		return Preflight{}, err
	}
	resolved, err := ResolveOptions(recipe, options)
	if err != nil {
		return Preflight{}, err
	}

	statuses, present, paths := Probe(recipe, resolved, nil)
	result := Preflight{RecipeID: recipe.ID, Tools: statuses, Installer: present["laravel"]}

	result.Ready = true
	for _, status := range statuses {
		if status.Required && !status.OK {
			result.Ready = false
			result.Notes = append(result.Notes, strings.TrimSpace(status.Label+" "+status.Problem))
		}
	}

	sample := map[string]string{
		"dir":         "my-app",
		"name":        "my_app",
		"python":      firstPath(paths["python"]),
		"venv_python": VenvPython(filepath.Join(".", "my-app")),
	}
	steps, err := BuildSteps(recipe, resolved, present, paths)
	if err != nil {
		return Preflight{}, err
	}
	for _, step := range steps {
		result.Commands = append(result.Commands, bindDisplay(step, sample))
	}

	if !present["laravel"] && resolved["starterKit"] != "" && resolved["starterKit"] != "none" {
		result.Notes = append(result.Notes,
			"The selected starter kit needs the Laravel installer. Locafy uses the Composer skeleton instead unless it is installed.")
	}
	return result, nil
}

// Start scaffolds an application inside an existing project. Nothing is saved
// until the installer's own files prove the framework was created.
func (s *Service) Start(projectID string, recipeID string, directory string, options map[string]string, tools map[string]string) (Job, error) {
	recipe, err := s.catalogue.Find(recipeID)
	if err != nil {
		return Job{}, err
	}
	resolved, err := ResolveOptions(recipe, options)
	if err != nil {
		return Job{}, err
	}
	project, err := projects.Get(s.database, projectID)
	if err != nil {
		return Job{}, err
	}

	relative, target, err := ResolveDirectory(project.RootPath, directory)
	if err != nil {
		return Job{}, err
	}
	state, err := InspectTarget(target)
	if err != nil {
		return Job{}, err
	}
	createdTarget := !state.Exists
	if state.Exists {
		if !state.Empty {
			return Job{}, ErrTargetNotEmpty
		}
		for _, command := range recipe.Commands {
			if command.Cwd == "parent" {
				return Job{}, fmt.Errorf("%w: %s installs into a folder it creates", ErrTargetNotEmpty, recipe.Label)
			}
		}
	}

	statuses, present, paths := Probe(recipe, resolved, tools)
	if err := requireReady(statuses); err != nil {
		return Job{}, err
	}

	steps, err := buildBoundSteps(recipe, resolved, present, paths, target)
	if err != nil {
		return Job{}, err
	}
	if !state.Exists && hasInsideSteps(recipe) {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return Job{}, fmt.Errorf("create %q: %w", target, err)
		}
		createdTarget = true
	}

	spec := RunSpec{
		ProjectID:     projectID,
		RecipeID:      recipe.ID,
		Directory:     relative,
		Target:        target,
		Steps:         steps,
		Verify:        recipe.Verify,
		CreatedTarget: createdTarget,
	}
	spec.OnSuccess = func(job Job) error {
		marker := ""
		if len(recipe.Verify) > 0 {
			marker = recipe.Verify[0]
		}
		input := applications.CreateInput{
			Name:            filepath.Base(target),
			Directory:       relative,
			Language:        recipe.Language,
			Framework:       recipe.Framework,
			Marker:          marker,
			StartCommand:    recipe.Application.StartCommand,
			StopCommand:     recipe.Application.StopCommand,
			Source:          applications.SourceDetected,
			ScaffoldRecipe:  recipe.ID,
			ScaffoldOptions: encodeOptions(resolved),
		}
		if _, err := applications.Create(s.database, projectID, input); err != nil {
			return err
		}
		return nil
	}
	return s.manager.Start(spec)
}

// StartProject creates a managed project from a recipe. The project row is
// written only after the scaffold succeeds, so a failed install registers
// nothing.
func (s *Service) StartProject(name string, recipeID string, options map[string]string, tools map[string]string) (Job, error) {
	recipe, err := s.catalogue.Find(recipeID)
	if err != nil {
		return Job{}, err
	}
	resolved, err := ResolveOptions(recipe, options)
	if err != nil {
		return Job{}, err
	}

	workspace, err := projects.WorkspaceDirectory(s.database)
	if err != nil {
		return Job{}, err
	}
	target, err := projects.AllocateManagedPath(workspace, name)
	if err != nil {
		return Job{}, err
	}

	statuses, present, paths := Probe(recipe, resolved, tools)
	if err := requireReady(statuses); err != nil {
		return Job{}, err
	}
	steps, err := buildBoundSteps(recipe, resolved, present, paths, target)
	if err != nil {
		return Job{}, err
	}
	// Recipes that run inside the folder (Django's virtual environment) need it
	// to exist; installers that create it themselves must find it absent.
	if hasInsideSteps(recipe) {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return Job{}, fmt.Errorf("create %q: %w", target, err)
		}
	}

	spec := RunSpec{
		RecipeID:      recipe.ID,
		Directory:     ".",
		Target:        target,
		Steps:         steps,
		Verify:        recipe.Verify,
		CreatedTarget: true,
	}
	spec.OnSuccess = func(job Job) error {
		project, err := projects.RegisterManagedAt(s.database, name, target)
		if err != nil {
			return err
		}
		if _, err := s.database.Exec(`UPDATE scaffold_jobs SET project_id = ? WHERE id = ?`, project.ID, job.ID); err != nil {
			return fmt.Errorf("record the scaffolded project: %w", err)
		}
		marker := ""
		if len(recipe.Verify) > 0 {
			marker = recipe.Verify[0]
		}
		input := applications.CreateInput{
			Name:            project.Name,
			Directory:       ".",
			Language:        recipe.Language,
			Framework:       recipe.Framework,
			Marker:          marker,
			StartCommand:    recipe.Application.StartCommand,
			StopCommand:     recipe.Application.StopCommand,
			Source:          applications.SourceDetected,
			ScaffoldRecipe:  recipe.ID,
			ScaffoldOptions: encodeOptions(resolved),
		}
		if _, err := applications.Create(s.database, project.ID, input); err != nil {
			return err
		}
		return nil
	}
	return s.manager.Start(spec)
}

// Job returns a job's state and output.
func (s *Service) Job(jobID string) (JobResult, error) {
	job, lines, err := s.manager.Get(jobID)
	if err != nil {
		return JobResult{}, err
	}
	return JobResult{Job: job, Lines: lines}, nil
}

// JobResult pairs a job with the output captured so far.
type JobResult struct {
	Job   Job
	Lines []string
}

// Cancel stops a running scaffold.
func (s *Service) Cancel(jobID string) error {
	return s.manager.Cancel(jobID)
}

// Discard removes a failed scaffold's folder when Locafy created it.
func (s *Service) Discard(jobID string) error {
	job, _, err := s.manager.Get(jobID)
	if err != nil {
		return err
	}
	if err := Discard(job); err != nil {
		return err
	}
	s.manager.Forget(jobID)
	return nil
}

// InstallLaravelInstaller installs the Laravel CLI at the user level through
// Composer. It is the one preparation step Locafy runs itself, and it never
// elevates.
func (s *Service) InstallLaravelInstaller() (string, error) {
	composer := ""
	if candidates := tools.Find("composer"); len(candidates) > 0 {
		composer = candidates[0].Path
	}
	if composer == "" {
		return "", fmt.Errorf("%w: Composer", ErrToolMissing)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	command := process.CommandFor(ctx, composer, []string{"global", "require", "laravel/installer", "--no-interaction"})
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("install the Laravel installer: %w", err)
	}
	return string(output), nil
}

func requireReady(statuses []ToolStatus) error {
	var missing []string
	for _, status := range statuses {
		if status.Required && !status.OK {
			label := status.Label + " " + status.Problem
			missing = append(missing, strings.TrimSpace(label))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrToolMissing, strings.Join(missing, "; "))
	}
	return nil
}

func hasInsideSteps(recipe Recipe) bool {
	for _, command := range recipe.Commands {
		if command.Cwd == "inside" {
			return true
		}
	}
	return false
}

func buildBoundSteps(recipe Recipe, options map[string]string, present map[string]bool, paths map[string]string, target string) ([]Step, error) {
	steps, err := BuildSteps(recipe, options, present, paths)
	if err != nil {
		return nil, err
	}
	base := filepath.Base(target)
	values := map[string]string{
		"dir":         base,
		"name":        Identifier(base),
		"python":      firstPath(paths["python"]),
		"venv_python": VenvPython(target),
	}
	if values["python"] == "" {
		values["python"] = "python"
	}
	for index := range steps {
		steps[index].Bin = bindText(steps[index].Bin, values)
		for argIndex := range steps[index].Args {
			steps[index].Args[argIndex] = bindText(steps[index].Args[argIndex], values)
		}
		display := append([]string{steps[index].Bin}, steps[index].Args...)
		steps[index].Display = strings.Join(display, " ")
	}
	return steps, nil
}

func bindText(value string, values map[string]string) string {
	for token, replacement := range values {
		value = strings.ReplaceAll(value, "{"+token+"}", replacement)
	}
	return value
}

func bindDisplay(step Step, values map[string]string) string {
	parts := []string{bindText(step.Bin, values)}
	for _, arg := range step.Args {
		parts = append(parts, bindText(arg, values))
	}
	return strings.Join(parts, " ")
}

func firstPath(path string) string {
	return path
}

func encodeOptions(options map[string]string) string {
	payload, err := json.Marshal(options)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func init() {
	// A catalogue that fails to load is reported by Preflight, but a developer
	// running the binary should see it immediately.
	if _, err := Load(); err != nil {
		fmt.Fprintln(os.Stderr, "locafy: scaffold catalogue:", err)
	}
}
