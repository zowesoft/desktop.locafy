package scaffold

import (
	"os"
	"path/filepath"
	"runtime"
)

// hostOS is the platform whose install locations are searched, so the tests can
// cover every platform from one machine.
var hostOS = runtime.GOOS

// knownLocations lists the places a tool's installers put an executable beside
// PATH, as globs. Each platform only searches its own locations: a Windows path
// means nothing on macOS or Linux, and an unset environment variable must never
// turn into a relative path under the application's working directory.
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

	// An unset base directory contributes nothing rather than an empty glob.
	compacted := locations[:0]
	for _, location := range locations {
		if location != "" {
			compacted = append(compacted, location)
		}
	}
	return compacted
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
