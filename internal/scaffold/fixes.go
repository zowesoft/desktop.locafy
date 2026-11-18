package scaffold

import (
	"fmt"
	"os/exec"
	"strings"
)

// ToolFix tells the operator how to satisfy a requirement that is missing or
// too old, so a blocked scaffold has an obvious next step.
type ToolFix struct {
	Summary   string
	Command   string
	Link      string
	LinkLabel string
}

// installHint is one tool's guidance. Commands are only included where the
// platform has a package manager that ships the tool; everything else points at
// the publisher's own download page.
type installHint struct {
	windows string
	darwin  string
	link    string
	label   string
	note    string
	// linuxNote warns where the distribution's package is usually older than
	// the version a recipe requires.
	linuxNote string
}

var installHints = map[string]installHint{
	"php": {
		windows: "winget install --id PHP.PHP.8.4 -e",
		darwin:  "brew install php",
		link:    "https://windows.php.net/download/",
		label:   "PHP downloads",
	},
	"composer": {
		link:  "https://getcomposer.org/download/",
		label: "Composer download",
		note:  "On Windows, Composer-Setup.exe installs Composer and adds it to PATH.",
	},
	"node": {
		windows:   "winget install --id OpenJS.NodeJS.LTS -e",
		darwin:    "brew install node",
		link:      "https://nodejs.org/en/download",
		label:     "Node.js downloads",
		linuxNote: "Distribution packages usually trail the required release, so install from the download page or NodeSource.",
	},
	"npm": {
		link:  "https://nodejs.org/en/download",
		label: "Node.js downloads",
		note:  "npm ships with Node.js; install Node.js and reopen Locafy.",
	},
	"python": {
		link:  "https://www.python.org/downloads/",
		label: "Python downloads",
	},
	"dotnet": {
		darwin: "brew install --cask dotnet-sdk",
		link:   "https://dotnet.microsoft.com/download",
		label:  ".NET downloads",
	},
}

// fixFor describes how to satisfy one requirement. installed is the version
// the probe found, empty when the tool is missing entirely.
func fixFor(tool string, label string, minVersion string, installed string) *ToolFix {
	label = strings.TrimSpace(label)
	if label == "" {
		label = tool
	}

	fix := &ToolFix{}
	if installed == "" {
		fix.Summary = fmt.Sprintf("%s %s or newer is required and Locafy could not find it.", label, minVersion)
	} else {
		fix.Summary = fmt.Sprintf("%s %s is installed, but %s or newer is required.", label, installed, minVersion)
	}

	hint, ok := installHints[tool]
	if !ok {
		return fix
	}

	// Version-specific package identifiers follow the minimum the recipe
	// states, so the suggested install satisfies the requirement.
	version := requirementSeries(minVersion)
	switch hostOS {
	case "windows":
		switch tool {
		case "python":
			if version != "" {
				fix.Command = "winget install --id Python.Python." + version + " -e"
			}
		case "dotnet":
			if major := strings.Split(version, ".")[0]; major != "" {
				fix.Command = "winget install --id Microsoft.DotNet.SDK." + major + " -e"
			}
		default:
			fix.Command = hint.windows
		}
	case "darwin":
		fix.Command = hint.darwin
	default:
		fix.Command = linuxCommand(tool, version)
		if hint.linuxNote != "" {
			fix.Summary += " " + hint.linuxNote
		}
	}

	if hint.note != "" {
		fix.Summary += " " + hint.note
	}
	fix.Link = hint.link
	fix.LinkLabel = hint.label
	if hint.link != "" {
		fix.Summary += " After installing, reopen Locafy so the new PATH is picked up."
	}
	return fix
}

// linuxPackageManager names the installer this machine actually has, so a
// suggested command is one the operator can run. It is a seam for tests.
var linuxPackageManager = func() string {
	for _, manager := range []struct{ binary, name string }{
		{"apt-get", "apt"},
		{"dnf", "dnf"},
		{"pacman", "pacman"},
		{"zypper", "zypper"},
		{"apk", "apk"},
	} {
		if _, err := exec.LookPath(manager.binary); err == nil {
			return manager.name
		}
	}
	return ""
}

// linuxCommand is the install command for a tool whose distribution package
// carries the required release. Node.js is left to its download page because
// distributions freeze a single major, and Composer and npm have no package of
// their own.
func linuxCommand(tool string, version string) string {
	manager := linuxPackageManager()
	if manager == "" {
		return ""
	}
	series := strings.TrimSpace(version)
	major := strings.Split(series, ".")[0]

	switch manager {
	case "apt":
		switch tool {
		case "php":
			return "sudo apt install php-cli"
		case "python":
			if series != "" {
				return "sudo apt install python3." + minorOf(series)
			}
		case "dotnet":
			if major != "" {
				return "sudo apt install dotnet-sdk-" + major + ".0"
			}
		}
	case "dnf":
		switch tool {
		case "php":
			return "sudo dnf install php-cli"
		case "python":
			if series != "" {
				return "sudo dnf install python3." + minorOf(series)
			}
		case "dotnet":
			if major != "" {
				return "sudo dnf install dotnet-sdk-" + major + ".0"
			}
		}
	case "pacman":
		switch tool {
		case "php":
			return "sudo pacman -S php"
		case "python":
			return "sudo pacman -S python"
		}
	case "zypper":
		switch tool {
		case "php":
			return "sudo zypper install php8"
		case "python":
			return "sudo zypper install python3"
		}
	case "apk":
		switch tool {
		case "php":
			return "sudo apk add php83"
		case "python":
			return "sudo apk add python3"
		}
	}
	return ""
}

// minorOf keeps the minor part of a major.minor requirement, which is how
// distributions name a Python series.
func minorOf(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version
	}
	return parts[1]
}

// requirementSeries keeps the major and minor parts of a minimum version,
// which is what package managers name their major releases by.
func requirementSeries(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}
