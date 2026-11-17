package applications

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Detection is an application the files in a folder suggest. It is advisory:
// nothing is executed or saved until the operator confirms it, and every field
// stays editable before that happens.
type Detection struct {
	Name         string
	Directory    string
	Language     string
	Framework    string
	Marker       string
	StartCommand string
}

// skippedDirectories hold dependencies, build output or tool state rather than
// applications of their own.
var skippedDirectories = map[string]bool{
	"node_modules": true, "vendor": true, ".git": true, ".next": true, "dist": true,
	"build": true, "bin": true, "obj": true, "target": true, "venv": true,
	".venv": true, "__pycache__": true, "coverage": true, "tmp": true, "temp": true,
}

// Detect proposes the applications inside a project: the root folder, its child
// folders and their children, so a monorepo laid out as apps/web or packages/api
// is found without walking a tree of dependencies. Only file names and a few
// small manifests are read.
func Detect(rootPath string) ([]Detection, error) {
	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		return nil, ErrDirectoryRequired
	}
	root, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", rootPath, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, ErrDirectoryNotDir
	}

	directories := []string{"."}
	children, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", root, err)
	}
	for _, child := range children {
		name := child.Name()
		if !child.IsDir() || skippedDirectory(name) {
			continue
		}
		directories = append(directories, name)

		grandchildren, err := os.ReadDir(filepath.Join(root, name))
		if err != nil {
			continue
		}
		for _, grandchild := range grandchildren {
			if !grandchild.IsDir() || skippedDirectory(grandchild.Name()) {
				continue
			}
			directories = append(directories, filepath.ToSlash(filepath.Join(name, grandchild.Name())))
		}
	}

	detections := make([]Detection, 0, len(directories))
	for _, directory := range directories {
		if detection, ok := detectIn(root, directory); ok {
			detections = append(detections, detection)
		}
	}
	return detections, nil
}

// detectIn names the application a single folder holds, if one is recognizable.
// The checks run in a fixed order so a folder carrying several markers gets one
// stable, predictable proposal rather than a random one.
func detectIn(root string, relative string) (Detection, bool) {
	absolute := root
	if relative != "." {
		absolute = filepath.Join(root, filepath.FromSlash(relative))
	}

	entries, err := os.ReadDir(absolute)
	if err != nil {
		return Detection{}, false
	}
	names := make(map[string]bool, len(entries))
	csproj, solution := "", ""
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		names[name] = true
		lower := strings.ToLower(name)
		if csproj == "" && strings.HasSuffix(lower, ".csproj") {
			csproj = name
		}
		if solution == "" && strings.HasSuffix(lower, ".sln") {
			solution = name
		}
	}

	base := filepath.Base(absolute)
	switch {
	case names["package.json"]:
		return detectNode(absolute, relative, base, names), true
	case names["composer.json"]:
		detection := Detection{Name: base, Directory: relative, Language: "PHP", Marker: "composer.json"}
		if names["artisan"] {
			detection.Framework = "Laravel"
			detection.StartCommand = "php artisan serve"
		}
		return detection, true
	case names["go.mod"]:
		return Detection{Name: base, Directory: relative, Language: "Go", Marker: "go.mod", StartCommand: "go run ."}, true
	case names["manage.py"]:
		return Detection{Name: base, Directory: relative, Language: "Python", Framework: "Django", Marker: "manage.py", StartCommand: "python manage.py runserver"}, true
	case names["pyproject.toml"], names["requirements.txt"]:
		marker := "requirements.txt"
		if names["pyproject.toml"] {
			marker = "pyproject.toml"
		}
		return Detection{Name: base, Directory: relative, Language: "Python", Marker: marker}, true
	case names["Gemfile"]:
		detection := Detection{Name: base, Directory: relative, Language: "Ruby", Marker: "Gemfile"}
		if fileExists(absolute, "config/application.rb") {
			detection.Framework = "Ruby on Rails"
			detection.StartCommand = "bin/rails server"
		}
		return detection, true
	case csproj != "":
		return Detection{Name: strings.TrimSuffix(csproj, filepath.Ext(csproj)), Directory: relative, Language: ".NET", Marker: csproj, StartCommand: "dotnet run"}, true
	case solution != "":
		return Detection{Name: strings.TrimSuffix(solution, filepath.Ext(solution)), Directory: relative, Language: ".NET", Marker: solution}, true
	}
	return Detection{}, false
}

// detectNode reads package.json for the name, language, framework and the
// script an operator would run in development.
func detectNode(absolute string, relative string, base string, names map[string]bool) Detection {
	var manifest struct {
		Name            string            `json:"name"`
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if data, err := os.ReadFile(filepath.Join(absolute, "package.json")); err == nil {
		_ = json.Unmarshal(data, &manifest)
	}

	has := func(dependency string) bool {
		_, runtime := manifest.Dependencies[dependency]
		_, development := manifest.DevDependencies[dependency]
		return runtime || development
	}

	name := strings.TrimSpace(manifest.Name)
	if name == "" {
		name = base
	}

	detection := Detection{Name: name, Directory: relative, Marker: "package.json", Language: "JavaScript"}
	if has("typescript") || names["tsconfig.json"] {
		detection.Language = "TypeScript"
	}

	for _, candidate := range []struct {
		dependency string
		framework  string
	}{
		{"next", "Next.js"},
		{"nuxt", "Nuxt"},
		{"@nestjs/core", "NestJS"},
		{"@angular/core", "Angular"},
		{"react-scripts", "React"},
		{"svelte", "Svelte"},
		{"vite", "Vite"},
		{"express", "Express"},
		{"fastify", "Fastify"},
		{"react", "React"},
		{"vue", "Vue"},
	} {
		if has(candidate.dependency) {
			detection.Framework = candidate.framework
			break
		}
	}

	switch {
	case manifest.Scripts["dev"] != "":
		detection.StartCommand = "npm run dev"
	case manifest.Scripts["start"] != "":
		detection.StartCommand = "npm start"
	}
	return detection
}

// skippedDirectory reports folders holding dependencies, build output or tool
// state rather than an application of their own.
func skippedDirectory(name string) bool {
	return skippedDirectories[name] || strings.HasPrefix(name, ".")
}

func fileExists(directory string, relative string) bool {
	info, err := os.Stat(filepath.Join(directory, filepath.FromSlash(relative)))
	return err == nil && !info.IsDir()
}
