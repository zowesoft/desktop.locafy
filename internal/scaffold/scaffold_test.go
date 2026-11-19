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
	venvPython := filepath.Join(".venv", "Scripts", "python.exe")
	if runtime.GOOS != "windows" {
		venvPython = filepath.Join(".venv", "bin", "python")
	}
	if !strings.Contains(filepath.ToSlash(steps[1].Bin), filepath.ToSlash(venvPython)) ||
		!strings.Contains(strings.Join(steps[1].Args, " "), "django~=5.2.0") {
		t.Errorf("install step = %+v", steps[1])
	}
	if !strings.Contains(filepath.ToSlash(steps[2].Bin), filepath.ToSlash(venvPython)) ||
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

func probePython(t *testing.T, version string, candidates []ToolCandidate) ToolStatus {
	t.Helper()
	django, err := catalogue(t).Find("django")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	restore := findToolFor
	findToolFor = func(name string) []ToolCandidate {
		if name == "python" {
			return candidates
		}
		return nil
	}
	t.Cleanup(func() { findToolFor = restore })

	statuses, _, _ := Probe(django, map[string]string{"version": version}, nil)
	for _, status := range statuses {
		if status.Tool == "python" {
			return status
		}
	}
	t.Fatal("the probe reported no python status")
	return ToolStatus{}
}

func pythonCandidates(versions ...string) []ToolCandidate {
	paths := map[string]string{
		"3.9.13":  `C:\Users\me\AppData\Local\Programs\Python\Python39\python.exe`,
		"3.10.11": `C:\laragon\bin\python\python-3.10\python.exe`,
		"3.12.7":  `C:\Program Files\Python312\python.exe`,
		"3.13.2":  `C:\Program Files\Python313\python.exe`,
	}
	candidates := make([]ToolCandidate, 0, len(versions))
	for _, version := range versions {
		candidates = append(candidates, ToolCandidate{Path: paths[version], Version: version, Source: "installed"})
	}
	return candidates
}

func TestProbePicksTheNewestPythonForDjango(t *testing.T) {
	status := probePython(t, "6.1", pythonCandidates("3.10.11", "3.9.13", "3.12.7", "3.13.2"))

	if status.MinVersion != "3.12" {
		t.Fatalf("minimum = %q, want 3.12 for Django 6.1", status.MinVersion)
	}
	if status.Version != "3.13.2" || !strings.Contains(status.Selected, "Python313") {
		t.Fatalf("selected %q (%q), want the newest interpreter", status.Selected, status.Version)
	}
	if !status.OK {
		t.Fatalf("problem = %q, want a usable interpreter", status.Problem)
	}
}

func TestProbeFlagsAPythonTooOldForTheChosenDjango(t *testing.T) {
	status := probePython(t, "6.1", pythonCandidates("3.9.13", "3.10.11"))

	if status.OK {
		t.Fatal("an interpreter below 3.12 was reported usable for Django 6.1")
	}
	if status.Problem != "needs 3.12 or newer" {
		t.Fatalf("problem = %q, want needs 3.12 or newer", status.Problem)
	}
	if status.Version != "3.10.11" {
		t.Fatalf("selected version = %q, want the newest copy named so the problem is clear", status.Version)
	}
}

func TestDjangoOlderVersionAcceptsAnOlderPython(t *testing.T) {
	status := probePython(t, "5.2", pythonCandidates("3.10.11", "3.9.13"))

	if status.MinVersion != "3.10" {
		t.Fatalf("minimum = %q, want 3.10 for Django 5.2", status.MinVersion)
	}
	if status.Version != "3.10.11" || !status.OK {
		t.Fatalf("selected %q (%q), problem %q, want 3.10 accepted", status.Selected, status.Version, status.Problem)
	}
}

func TestAnOldInterpreterComesWithAWayToFixIt(t *testing.T) {
	status := probePython(t, "6.1", pythonCandidates("3.9.13"))

	if status.Fix == nil {
		t.Fatal("a blocked requirement carries no fix")
	}
	if !strings.Contains(status.Fix.Summary, "3.12") || !strings.Contains(status.Fix.Summary, "3.9.13") {
		t.Fatalf("summary = %q, want the installed and required versions", status.Fix.Summary)
	}
	if status.Fix.Link == "" || status.Fix.LinkLabel == "" {
		t.Fatalf("fix = %+v, want an official download link", status.Fix)
	}
	if runtime.GOOS == "windows" && !strings.Contains(status.Fix.Command, "Python.Python.3.12") {
		t.Fatalf("command = %q, want the winget package for the required Python", status.Fix.Command)
	}
}

func TestAMissingToolComesWithAWayToFixIt(t *testing.T) {
	status := probePython(t, "6.1", nil)

	if status.Problem != "not installed" {
		t.Fatalf("problem = %q, want not installed", status.Problem)
	}
	if status.Fix == nil {
		t.Fatal("a missing tool carries no fix")
	}
	if !strings.Contains(status.Fix.Summary, "could not find it") {
		t.Fatalf("summary = %q, want an explanation that nothing was found", status.Fix.Summary)
	}
	if status.Fix.Link == "" {
		t.Fatal("a missing tool has no download link")
	}
}

func TestToolsWithoutAPackageManagerStillPointAtTheirDownloadPage(t *testing.T) {
	composer := fixFor("composer", "Composer", "2.2", "1.10.0")
	if composer.Link != "https://getcomposer.org/download/" {
		t.Fatalf("composer link = %q, want the official download page", composer.Link)
	}
	if composer.Command != "" {
		t.Fatalf("composer command = %q, want no command where there is no package to install", composer.Command)
	}
	if !strings.Contains(composer.Summary, "1.10.0") {
		t.Fatalf("summary = %q, want the installed version named", composer.Summary)
	}

	npm := fixFor("npm", "npm", "10", "")
	if !strings.Contains(npm.Summary, "ships with Node.js") {
		t.Fatalf("npm summary = %q, want it to point at Node.js", npm.Summary)
	}
	if npm.Link != "https://nodejs.org/en/download" {
		t.Fatalf("npm link = %q, want the Node.js download page", npm.Link)
	}
}

func TestRequiredToolsEachCarryAFix(t *testing.T) {
	for _, tool := range []string{"php", "composer", "node", "npm", "python", "dotnet"} {
		fix := fixFor(tool, tool, "1.0", "")
		if fix == nil || fix.Link == "" {
			t.Fatalf("%s fix = %+v, want a link", tool, fix)
		}
		if !strings.Contains(fix.Summary, "1.0") {
			t.Fatalf("%s summary = %q, want the required version", tool, fix.Summary)
		}
	}
}

func TestLinuxInstallsNameTheDistributionsPackage(t *testing.T) {
	restoreOS, restoreManager := hostOS, linuxPackageManager
	t.Cleanup(func() {
		hostOS = restoreOS
		linuxPackageManager = restoreManager
	})
	hostOS = "linux"
	linuxPackageManager = func() string { return "apt" }

	python := fixFor("python", "Python", "3.12", "3.10.6")
	if python.Command != "sudo apt install python3.12" {
		t.Errorf("python command = %q, want the distribution package for the required series", python.Command)
	}
	dotnet := fixFor("dotnet", ".NET SDK", "10", "")
	if dotnet.Command != "sudo apt install dotnet-sdk-10.0" {
		t.Errorf("dotnet command = %q, want the package for the required major", dotnet.Command)
	}
	node := fixFor("node", "Node.js", "20.19", "18.20.0")
	if node.Command != "" {
		t.Errorf("node command = %q, want the download page because distributions freeze one major", node.Command)
	}
	if !strings.Contains(node.Summary, "Distribution packages") || node.Link != "https://nodejs.org/en/download" {
		t.Errorf("node fix = %+v, want a note and the download page", node)
	}

	linuxPackageManager = func() string { return "" }
	php := fixFor("php", "PHP", "8.3", "")
	if php.Command != "" || php.Link != "https://windows.php.net/download/" {
		t.Errorf("php fix without a package manager = %+v, want the download page only", php)
	}
}
