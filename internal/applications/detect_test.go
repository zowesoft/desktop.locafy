package applications

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, root string, path string, content string) {
	t.Helper()

	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("create %q: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", full, err)
	}
}

func detectOne(t *testing.T, root string) Detection {
	t.Helper()

	detections, err := Detect(root)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(detections) != 1 {
		t.Fatalf("detections = %+v, want exactly one", detections)
	}
	return detections[0]
}

func TestDetectsNodeApplicationFromPackageJSON(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{
		"name": "storefront",
		"scripts": {"dev": "vite", "start": "node server.js"},
		"dependencies": {"react": "^19.0.0"},
		"devDependencies": {"vite": "^7.0.0"}
	}`)

	detection := detectOne(t, root)

	if detection.Language != "JavaScript" || detection.Framework != "Vite" {
		t.Fatalf("detection = %+v, want a JavaScript Vite application", detection)
	}
	if detection.Name != "storefront" || detection.Marker != "package.json" {
		t.Fatalf("detection = %+v, want the manifest name and marker", detection)
	}
	// A dev script wins over start: it is what an operator runs while working.
	if detection.StartCommand != "npm run dev" {
		t.Fatalf("start command = %q, want npm run dev", detection.StartCommand)
	}
	if detection.Directory != "." {
		t.Fatalf("directory = %q, want the project root", detection.Directory)
	}
}

func TestDetectsTypeScriptNextApplication(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"dependencies": {"next": "^15.0.0", "react": "^19.0.0"}}`)
	writeFile(t, root, "tsconfig.json", `{}`)

	detection := detectOne(t, root)

	if detection.Language != "TypeScript" || detection.Framework != "Next.js" {
		t.Fatalf("detection = %+v, want a TypeScript Next.js application", detection)
	}
	// No dev or start script: nothing is proposed rather than an invented one.
	if detection.StartCommand != "" {
		t.Fatalf("start command = %q, want none without a script", detection.StartCommand)
	}
}

func TestDetectsLaravelApplication(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "composer.json", `{"require": {"laravel/framework": "^11.0"}}`)
	writeFile(t, root, "artisan", `#!/usr/bin/env php`)

	detection := detectOne(t, root)

	if detection.Language != "PHP" || detection.Framework != "Laravel" {
		t.Fatalf("detection = %+v, want a PHP Laravel application", detection)
	}
	if detection.StartCommand != "php artisan serve" {
		t.Fatalf("start command = %q, want php artisan serve", detection.StartCommand)
	}
}

func TestDetectsGoApplication(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/api\n")

	detection := detectOne(t, root)

	if detection.Language != "Go" || detection.Marker != "go.mod" {
		t.Fatalf("detection = %+v, want a Go application from go.mod", detection)
	}
	if detection.StartCommand != "go run ." {
		t.Fatalf("start command = %q, want go run .", detection.StartCommand)
	}
}

func TestDetectsDjangoApplication(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "manage.py", "#!/usr/bin/env python\n")
	writeFile(t, root, "requirements.txt", "Django==5.0\n")

	detection := detectOne(t, root)

	if detection.Language != "Python" || detection.Framework != "Django" {
		t.Fatalf("detection = %+v, want a Python Django application", detection)
	}
	if detection.StartCommand != "python manage.py runserver" {
		t.Fatalf("start command = %q, want the Django development server", detection.StartCommand)
	}
}

func TestDetectsPlainPythonApplication(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "requirements.txt", "fastapi\n")

	detection := detectOne(t, root)

	if detection.Language != "Python" || detection.Framework != "" {
		t.Fatalf("detection = %+v, want plain Python", detection)
	}
	if detection.StartCommand != "" {
		t.Fatalf("start command = %q, want none guessed without a manifest command", detection.StartCommand)
	}
}

func TestDetectsRubyOnRailsApplication(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Gemfile", "source \"https://rubygems.org\"\n")
	writeFile(t, root, "config/application.rb", "module Demo\nend\n")

	detection := detectOne(t, root)

	if detection.Language != "Ruby" || detection.Framework != "Ruby on Rails" {
		t.Fatalf("detection = %+v, want a Rails application", detection)
	}
	if detection.StartCommand != "bin/rails server" {
		t.Fatalf("start command = %q, want bin/rails server", detection.StartCommand)
	}
}

func TestDetectsDotNetApplication(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Storefront.csproj", "<Project Sdk=\"Microsoft.NET.Sdk\"></Project>")

	detection := detectOne(t, root)

	if detection.Language != ".NET" || detection.Marker != "Storefront.csproj" {
		t.Fatalf("detection = %+v, want a .NET application from the project file", detection)
	}
	if detection.Name != "Storefront" || detection.StartCommand != "dotnet run" {
		t.Fatalf("detection = %+v, want the project name and dotnet run", detection)
	}
}

func TestDetectsApplicationsNestedOneLevelDeep(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"name": "workspace-root"}`)
	writeFile(t, root, "apps/storefront/composer.json", `{"require": {"laravel/framework": "^11.0"}}`)
	writeFile(t, root, "apps/reporting/go.mod", "module example.com/reporting\n")

	detections, err := Detect(root)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(detections) != 3 {
		t.Fatalf("detections = %+v, want the root and both nested applications", detections)
	}

	byDirectory := make(map[string]Detection, len(detections))
	for _, detection := range detections {
		byDirectory[detection.Directory] = detection
	}
	if byDirectory["."].Name != "workspace-root" {
		t.Errorf("root detection = %+v, want the root package", byDirectory["."])
	}
	// The child folder's own name is used: the monorepo's packages are separate
	// applications, not one application at the root.
	if byDirectory["apps/storefront"].Language != "PHP" {
		t.Errorf("storefront = %+v, want PHP", byDirectory["apps/storefront"])
	}
	if byDirectory["apps/reporting"].Language != "Go" {
		t.Errorf("reporting = %+v, want Go", byDirectory["apps/reporting"])
	}
}

func TestSkipsDependenciesAndHiddenFolders(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "node_modules/left-pad/package.json", `{"name": "left-pad"}`)
	writeFile(t, root, ".cache/package.json", `{"name": "cache"}`)
	writeFile(t, root, "api/go.mod", "module example.com/api\n")

	detections, err := Detect(root)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(detections) != 1 || detections[0].Directory != "api" {
		t.Fatalf("detections = %+v, want only the api application", detections)
	}
}

func TestDetectsNothingInAFolderWithoutMarkers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# nothing to run yet\n")

	detections, err := Detect(root)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(detections) != 0 {
		t.Fatalf("detections = %+v, want none", detections)
	}
}

func TestDetectRefusesAPathThatIsNotAFolder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{}`)

	if _, err := Detect(filepath.Join(root, "package.json")); err != ErrDirectoryNotDir {
		t.Fatalf("err = %v, want the not-a-folder error", err)
	}
}
