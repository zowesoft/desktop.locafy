package scaffold

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func catalogue(t *testing.T) *Catalogue {
	t.Helper()
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loaded
}

func TestCatalogueShipsTheFiveOfficialScaffolders(t *testing.T) {
	loaded := catalogue(t)

	var ids []string
	for _, recipe := range loaded.Recipes {
		ids = append(ids, recipe.ID)
	}
	want := []string{"laravel", "nextjs", "vite", "django", "dotnet"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("recipes = %v, want %v", ids, want)
	}
}

func TestEveryRecipeOffersItsLatestVersion(t *testing.T) {
	loaded := catalogue(t)
	wantLatest := map[string]string{
		"laravel": "13",
		"nextjs":  "16",
		"vite":    "9",
		"django":  "6.1",
		"dotnet":  "11.0",
	}
	for _, recipe := range loaded.Recipes {
		key := "version"
		if recipe.ID == "vite" {
			key = "createVite"
		}
		option := findOption(t, recipe, key)
		if option.Default != wantLatest[recipe.ID] {
			t.Errorf("%s default version = %q, want %q", recipe.ID, option.Default, wantLatest[recipe.ID])
		}
		if len(option.Values) < 2 {
			t.Errorf("%s offers %d versions, want a choice", recipe.ID, len(option.Values))
		}
	}
}

func findOption(t *testing.T, recipe Recipe, key string) Option {
	t.Helper()
	for _, option := range recipe.Options {
		if option.Key == key {
			return option
		}
	}
	t.Fatalf("%s has no option %q", recipe.ID, key)
	return Option{}
}

func TestLaravelUsesInstallerWhenPresent(t *testing.T) {
	loaded := catalogue(t)
	recipe, _ := loaded.Find("laravel")
	options, err := ResolveOptions(recipe, map[string]string{
		"version": "13", "starterKit": "react", "database": "sqlite",
	})
	if err != nil {
		t.Fatalf("ResolveOptions: %v", err)
	}

	steps, err := BuildSteps(recipe, options, map[string]bool{"laravel": true, "php": true, "composer": true},
		map[string]string{"laravel": `C:\tools\laravel.bat`, "php": `C:\php\php.exe`})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	if steps[0].Bin != `C:\tools\laravel.bat` {
		t.Errorf("bin = %q, want the installer", steps[0].Bin)
	}
	joined := strings.Join(steps[0].Args, " ")
	for _, want := range []string{"new", "{dir}", "--no-interaction", "--database=sqlite", "--react"} {
		if !strings.Contains(joined, want) {
			t.Errorf("installer args %q missing %q", joined, want)
		}
	}
}

func TestLaravelFallsBackToComposerWithoutInstaller(t *testing.T) {
	loaded := catalogue(t)
	recipe, _ := loaded.Find("laravel")
	options, _ := ResolveOptions(recipe, map[string]string{"version": "13", "starterKit": "none", "database": "sqlite"})

	steps, err := BuildSteps(recipe, options, map[string]bool{"php": true, "composer": true},
		map[string]string{"composer": `C:\composer.bat`})
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}

	if len(steps) != 1 || steps[0].Bin != `C:\composer.bat` {
		t.Fatalf("steps = %+v, want one composer step", steps)
	}
	joined := strings.Join(steps[0].Args, " ")
	if !strings.Contains(joined, "laravel/laravel:^13.0") {
		t.Errorf("composer args = %q, want the pinned constraint", joined)
	}
}

func TestLaravelVersionPinUsesComposerEvenWithInstaller(t *testing.T) {
	loaded := catalogue(t)
	recipe, _ := loaded.Find("laravel")
	options, _ := ResolveOptions(recipe, map[string]string{"version": "12", "starterKit": "none", "database": "sqlite"})

	steps, err := BuildSteps(recipe, options, map[string]bool{"laravel": true, "php": true, "composer": true}, nil)
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	if steps[0].Bin != "composer" {
		t.Fatalf("bin = %q, want composer for a pinned older version", steps[0].Bin)
	}
	if !strings.Contains(strings.Join(steps[0].Args, " "), "laravel/laravel:^12.0") {
		t.Errorf("args = %q", steps[0].Args)
	}
}

func TestNextVersionPinsTheScaffolder(t *testing.T) {
	loaded := catalogue(t)
	recipe, _ := loaded.Find("nextjs")

	for version, want := range map[string]string{"16": "create-next-app@16", "15": "create-next-app@15"} {
		options, _ := ResolveOptions(recipe, map[string]string{"version": version})
		steps, err := BuildSteps(recipe, options, map[string]bool{"npx": true}, nil)
		if err != nil {
			t.Fatalf("BuildSteps: %v", err)
		}
		if !strings.Contains(strings.Join(steps[0].Args, " "), want) {
			t.Errorf("Next %s args = %q, want %q", version, steps[0].Args, want)
		}
	}
}

func TestViteChoiceMapsToTheCreateViteRelease(t *testing.T) {
	loaded := catalogue(t)
	recipe, _ := loaded.Find("vite")

	options, _ := ResolveOptions(recipe, map[string]string{"createVite": "8", "template": "vue-ts"})
	steps, err := BuildSteps(recipe, options, map[string]bool{"npx": true, "npm": true}, nil)
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	joined := strings.Join(steps[0].Args, " ")
	if !strings.Contains(joined, "create-vite@8") || !strings.Contains(joined, "--template vue-ts") {
		t.Errorf("args = %q", joined)
	}
	if len(steps) != 2 || steps[1].Cwd != "inside" || strings.Join(steps[1].Args, " ") != "install" {
		t.Errorf("steps = %+v, want an npm install inside the new app", steps)
	}
}

func TestDjangoRunsInAVirtualEnvironment(t *testing.T) {
	loaded := catalogue(t)
	recipe, _ := loaded.Find("django")
	options, _ := ResolveOptions(recipe, map[string]string{"version": "5.2"})

	steps, err := buildBoundSteps(recipe, options, map[string]bool{"python": true}, map[string]string{"python": `C:\Python312\python.exe`}, filepath.Join(t.TempDir(), "blog"))
	if err != nil {
		t.Fatalf("BuildSteps: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(steps))
	}
	if steps[0].Bin != `C:\Python312\python.exe` || strings.Join(steps[0].Args, " ") != "-m venv .venv" {
		t.Errorf("first step = %+v", steps[0])
	}
	if !strings.Contains(filepath.ToSlash(steps[1].Bin), ".venv/Scripts/python.exe") ||
		!strings.Contains(strings.Join(steps[1].Args, " "), "django~=5.2.0") {
		t.Errorf("install step = %+v", steps[1])
	}
	if !strings.Contains(filepath.ToSlash(steps[2].Bin), ".venv/Scripts/python.exe") ||
		!strings.Contains(strings.Join(steps[2].Args, " "), "startproject config .") {
		t.Errorf("startproject step = %+v", steps[2])
	}
}

func TestBoundStepsReplaceEveryPlaceholder(t *testing.T) {
	loaded := catalogue(t)
	recipe, _ := loaded.Find("django")
	options, _ := ResolveOptions(recipe, map[string]string{"version": "5.2"})
	steps, err := buildBoundSteps(recipe, options, map[string]bool{"python": true}, map[string]string{"python": "python"}, filepath.Join(t.TempDir(), "blog"))
	if err != nil {
		t.Fatalf("buildBoundSteps: %v", err)
	}
	for _, step := range steps {
		if strings.Contains(step.Bin, "{") {
			t.Errorf("bin still has a token: %q", step.Bin)
		}
		for _, arg := range step.Args {
			if strings.Contains(arg, "{") {
				t.Errorf("argument still has a token: %q", arg)
			}
		}
	}
	if !strings.HasSuffix(steps[1].Bin, "python.exe") && runtime.GOOS == "windows" {
		t.Errorf("venv python = %q", steps[1].Bin)
	}
}

func TestUnsafeOrUnknownOptionsAreRejected(t *testing.T) {
	loaded := catalogue(t)
	recipe, _ := loaded.Find("laravel")

	if _, err := ResolveOptions(recipe, map[string]string{"version": "11"}); err == nil {
		t.Error("a version that was never offered was accepted")
	}
	if _, err := ResolveOptions(recipe, map[string]string{"command": "rm -rf /"}); err == nil {
		t.Error("an unknown option was accepted")
	}
}

func TestCatalogueRejectsShellCharactersInArguments(t *testing.T) {
	bad := &Catalogue{Recipes: []Recipe{{
		ID: "bad", Label: "Bad", Commands: []Command{{Bin: "npm", Cwd: "parent", Args: []Arg{{Literal: "install && rm -rf /"}}}},
		Verify: []string{"package.json"}, Application: ApplicationSpec{StartCommand: "npm run dev"},
	}}}
	if err := bad.validate(); err == nil {
		t.Fatal("an argument with a shell operator was accepted")
	}
}

func TestCatalogueRejectsUnknownTokens(t *testing.T) {
	bad := &Catalogue{Recipes: []Recipe{{
		ID: "bad", Label: "Bad", Commands: []Command{{Bin: "npm", Cwd: "parent", Args: []Arg{{Literal: "{surprise}"}}}},
		Verify: []string{"package.json"}, Application: ApplicationSpec{StartCommand: "npm run dev"},
	}}}
	if err := bad.validate(); err == nil {
		t.Fatal("an unknown token was accepted")
	}
}

func TestResolveDirectoryKeepsTheTargetInsideTheProject(t *testing.T) {
	root := t.TempDir()

	relative, absolute, err := ResolveDirectory(root, "apps/web")
	if err != nil {
		t.Fatalf("ResolveDirectory: %v", err)
	}
	if relative != "apps/web" || !strings.HasSuffix(filepath.ToSlash(absolute), "apps/web") {
		t.Errorf("got %q and %q", relative, absolute)
	}

	if _, _, err := ResolveDirectory(root, "../outside"); err == nil {
		t.Error("a folder outside the project was accepted")
	}
	if _, _, err := ResolveDirectory(root, "."); err == nil {
		t.Error("the project root itself was accepted as an application folder")
	}
}

func TestInspectTargetSeesEmptyAndFilledFolders(t *testing.T) {
	target := t.TempDir()
	state, err := InspectTarget(target)
	if err != nil || !state.Exists || !state.Empty {
		t.Fatalf("state = %+v, err = %v", state, err)
	}

	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err = InspectTarget(target)
	if err != nil || !state.Exists || state.Empty {
		t.Fatalf("state = %+v, err = %v", state, err)
	}

	state, err = InspectTarget(filepath.Join(target, "missing"))
	if err != nil || state.Exists {
		t.Fatalf("missing target state = %+v, err = %v", state, err)
	}
}

func TestVersionComparison(t *testing.T) {
	if !AtLeast("8.3.7", "8.3") {
		t.Error("8.3.7 should satisfy 8.3")
	}
	if AtLeast("7.4.9", "8.3") {
		t.Error("7.4.9 should not satisfy 8.3")
	}
	if got := parseVersion("PHP 8.4.18 (cli)"); got != "8.4.18" {
		t.Errorf("parseVersion = %q", got)
	}
	if got := parseVersion("Composer version 2.8.8 2025-04-04"); got != "2.8.8" {
		t.Errorf("parseVersion = %q", got)
	}
}

func TestSlugAndIdentifier(t *testing.T) {
	if got := Slugify("My Cool App"); got != "my-cool-app" {
		t.Errorf("Slugify = %q", got)
	}
	if got := Identifier("My-App"); got != "my_app" {
		t.Errorf("Identifier = %q", got)
	}
	if got := Identifier("2fast"); got != "app_2fast" {
		t.Errorf("Identifier = %q", got)
	}
}

func TestOptionalToolPrefersComposerOverAnEnvironmentWrapper(t *testing.T) {
	herd := &ToolCandidate{Path: `C:\Users\me\.config\herd\bin\laravel.bat`, Version: "5.25.3", Source: "PATH"}
	composer := &ToolCandidate{Path: `C:\Users\me\AppData\Roaming\Composer\vendor\bin\laravel.bat`, Version: "5.25.3", Source: "PATH"}
	if !betterCandidate(composer, herd, "") {
		t.Error("the Composer install should win over Herd's wrapper for the same version")
	}
}

func TestRequiredToolPrefersTheNewestVersion(t *testing.T) {
	older := &ToolCandidate{Path: `C:\laragon\bin\php\php-8.2\php.exe`, Version: "8.2.19", Source: "installed"}
	newer := &ToolCandidate{Path: `C:\laragon\bin\php\php-8.4\php.exe`, Version: "8.4.18", Source: "installed"}
	if !betterCandidate(newer, older, "8.3") {
		t.Error("a newer PHP should win when it meets the minimum")
	}
}
