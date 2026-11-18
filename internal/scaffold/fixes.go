package scaffold

import (
	"fmt"
	"runtime"
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
		windows: "winget install --id OpenJS.NodeJS.LTS -e",
		darwin:  "brew install node",
		link:    "https://nodejs.org/en/download",
		label:   "Node.js downloads",
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
	switch runtime.GOOS {
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
	}

	if hint.note != "" {
		fix.Summary += " " + hint.note
	}
	fix.Link = hint.link
	fix.LinkLabel = hint.label
	if fix.Command != "" {
		fix.Summary += " After installing, reopen Locafy so the new PATH is picked up."
	}
	return fix
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
