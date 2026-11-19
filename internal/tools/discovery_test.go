package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func locationsInclude(locations []string, want string) bool {
	// Separators are normalised on both sides because a platform's own
	// locations are simulated on whichever host runs the test.
	normalize := func(path string) string { return strings.ReplaceAll(path, `\`, "/") }
	for _, location := range locations {
		if strings.EqualFold(normalize(location), normalize(want)) {
			return true
		}
	}
	return false
}

func TestEachPlatformSearchesItsOwnInstallLocations(t *testing.T) {
	restore := hostOS
	t.Cleanup(func() { hostOS = restore })
	t.Setenv("APPDATA", `C:\Users\me\AppData\Roaming`)
	t.Setenv("LOCALAPPDATA", `C:\Users\me\AppData\Local`)
	t.Setenv("ProgramFiles", `C:\Program Files`)

	hostOS = "windows"
	for _, want := range []string{
		`C:\laragon\bin\python\python-*\python.exe`,
		`C:\Users\me\AppData\Roaming\Composer\vendor\bin\composer.bat`,
		`C:\Program Files\nodejs\npm.cmd`,
		`C:\Program Files\dotnet\dotnet.exe`,
	} {
		if !locationsInclude(windowsLocations("python"), want) && !locationsInclude(windowsLocations("composer"), want) &&
			!locationsInclude(windowsLocations("npm"), want) && !locationsInclude(windowsLocations("dotnet"), want) {
			t.Errorf("Windows searches no %q", want)
		}
	}
	for _, tool := range []string{"php", "composer", "node", "dotnet", "python"} {
		for _, location := range windowsLocations(tool) {
			if strings.HasPrefix(location, "/opt/") || strings.HasPrefix(location, "/usr/") {
				t.Errorf("Windows %s search includes the Unix path %q", tool, location)
			}
		}
	}

	hostOS = "darwin"
	if !locationsInclude(darwinLocations("php"), "/opt/homebrew/bin/php") {
		t.Error("macOS does not search Homebrew's php")
	}
	if !locationsInclude(darwinLocations("composer"), filepath.Join(homeDirectory(), ".config", "composer", "vendor", "bin", "composer")) {
		t.Error("macOS does not search Composer's global bin")
	}
	if !locationsInclude(darwinLocations("node"), filepath.Join(homeDirectory(), ".nvm", "versions", "node", "*", "bin", "node")) {
		t.Error("macOS does not search nvm's node versions")
	}
	if !locationsInclude(darwinLocations("python"), "/opt/homebrew/bin/python3.*") {
		t.Error("macOS does not search versioned Homebrew interpreters")
	}
	// Home-derived entries follow the machine running the test, so only the
	// fixed locations can be held to the simulated platform.
	home := homeDirectory()
	for _, tool := range []string{"php", "composer", "node", "dotnet", "python"} {
		for _, location := range darwinLocations(tool) {
			if strings.HasPrefix(location, home) {
				continue
			}
			if strings.Contains(location, `C:\`) || strings.Contains(location, "Program Files") {
				t.Errorf("macOS %s search includes the Windows path %q", tool, location)
			}
		}
	}

	hostOS = "linux"
	if !locationsInclude(linuxLocations("php"), "/usr/bin/php8.*") {
		t.Error("Linux does not search the distribution's versioned php")
	}
	if !locationsInclude(linuxLocations("composer"), filepath.Join(homeDirectory(), ".config", "composer", "vendor", "bin", "composer")) {
		t.Error("Linux does not search Composer's global bin")
	}
	if !locationsInclude(linuxLocations("dotnet"), "/usr/lib/dotnet/dotnet") {
		t.Error("Linux does not search the package-managed .NET root")
	}
	if !locationsInclude(linuxLocations("python"), "/usr/bin/python3.*") {
		t.Error("Linux does not search versioned interpreters")
	}
	for _, tool := range []string{"php", "composer", "node", "dotnet", "python"} {
		for _, location := range linuxLocations(tool) {
			if strings.HasPrefix(location, home) {
				continue
			}
			if strings.HasPrefix(location, "/opt/homebrew") {
				t.Errorf("Linux %s search includes a macOS-only path %q", tool, location)
			}
		}
	}
}

// absoluteFor reports whether a path is absolute on the platform whose
// locations are checked, which is not necessarily the host running the test.
func absoluteFor(platform string, path string) bool {
	if platform == "windows" {
		if strings.HasPrefix(path, `\\`) {
			return true
		}
		return len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
	}
	return strings.HasPrefix(path, "/")
}

func TestEveryLocationIsAbsoluteWithoutInstallVariables(t *testing.T) {
	restore := hostOS
	t.Cleanup(func() { hostOS = restore })
	for _, variable := range []string{"HOME", "APPDATA", "LOCALAPPDATA", "ProgramFiles", "ProgramFiles(x86)"} {
		t.Setenv(variable, "")
	}

	for _, platform := range []string{"windows", "darwin", "linux"} {
		hostOS = platform
		for _, tool := range []string{"php", "composer", "laravel", "node", "npm", "npx", "go", "java", "ruby", "dotnet", "python"} {
			for _, location := range knownLocations(tool) {
				prefix := location
				if index := strings.IndexAny(prefix, "*?"); index >= 0 {
					prefix = prefix[:index]
				}
				// Home-derived entries follow the machine running the test, so a
				// location may be absolute for the simulated platform or for the
				// host, but it must be absolute for one of them.
				if !absoluteFor(platform, prefix) && !filepath.IsAbs(prefix) {
					t.Errorf("%s %s location %q is relative", platform, tool, location)
				}
			}
		}
	}
}

func TestUnixAsksForPython3AndWindowsForPython(t *testing.T) {
	restore := hostOS
	t.Cleanup(func() { hostOS = restore })

	hostOS = "windows"
	if got := strings.Join(searchNames("python"), ","); got != "python" {
		t.Errorf("Windows names = %q, want python", got)
	}
	for _, platform := range []string{"darwin", "linux"} {
		hostOS = platform
		if got := strings.Join(searchNames("python"), ","); got != "python,python3" {
			t.Errorf("%s names = %q, want python and python3", platform, got)
		}
	}
	hostOS = "linux"
	if got := strings.Join(searchNames("php"), ","); got != "php" {
		t.Errorf("php names = %q, want php", got)
	}
}

func TestVersionGlobsDoNotStandInForAnInterpreter(t *testing.T) {
	for _, path := range []string{"/opt/homebrew/bin/python3.12", "/usr/bin/python3", `C:\Python312\python.exe`} {
		if !usableCopy("python", path) {
			t.Errorf("%q was rejected as an interpreter", path)
		}
	}
	for _, path := range []string{"/opt/homebrew/bin/python3.12-config", "/usr/bin/python3.12-dbg.py", "/usr/bin/python3-config"} {
		if usableCopy("python", path) {
			t.Errorf("%q was accepted as an interpreter", path)
		}
	}
	if !usableCopy("php", "/usr/bin/php8.3") {
		t.Error("only python locations need the interpreter filter")
	}
}

func TestCompareVersions(t *testing.T) {
	if !AtLeast("8.3.7", "8.3") {
		t.Error("8.3.7 is not at least 8.3")
	}
	if AtLeast("7.4.9", "8.3") {
		t.Error("7.4.9 is at least 8.3")
	}
	if AtLeast("8.3", "8.3.1") {
		t.Error("8.3 is at least 8.3.1")
	}
	if CompareVersions("8.3", "8.3") != 0 {
		t.Error("8.3 does not equal itself")
	}
}

func TestParseVersion(t *testing.T) {
	if got := parseVersion("PHP 8.4.18 (cli)"); got != "8.4.18" {
		t.Errorf("parseVersion = %q, want 8.4.18", got)
	}
	if got := parseVersion("Composer version 2.8.8 2025-04-04"); got != "2.8.8" {
		t.Errorf("parseVersion = %q, want 2.8.8", got)
	}
	if got := parseVersion("no version here"); got != "" {
		t.Errorf("parseVersion = %q, want empty", got)
	}
}

func TestPyLauncherOutputYieldsEveryInterpreter(t *testing.T) {
	output := "Installed Pythons found by C:\\Windows\\py.exe Launcher for Windows *\r\n\r\n" +
		" -3.9-64        C:\\Users\\me\\AppData\\Local\\Programs\\Python\\Python39\\python.exe\r\n" +
		" -V:3.12 *      C:\\Program Files\\Python312\\python.exe\r\n"

	paths := parsePyLauncher(output)
	want := []string{
		`C:\Users\me\AppData\Local\Programs\Python\Python39\python.exe`,
		`C:\Program Files\Python312\python.exe`,
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Fatalf("paths = %v, want %v", paths, want)
		}
	}
}
