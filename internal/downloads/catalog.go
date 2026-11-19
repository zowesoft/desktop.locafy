// Package downloads runs Locafy's install jobs: it downloads a portable
// archive into a directory Locafy owns, verifies it and unpacks it, so a
// managed runtime exists without elevation, system PATH writes or an
// uninstaller. Jobs are machine-scoped rows with progress, outcome and
// retry (ADR-0019).
package downloads

import (
	"runtime"
)

// Spec describes one installable archive: where it comes from, how it is
// unpacked, and what proves the unpack worked.
type Spec struct {
	Kind    string `json:"kind"`
	Version string `json:"version"`
	URL     string `json:"url"`
	// Format is the archive's container: zip or targz.
	Format string `json:"format"`
	// MustContain is the slash-separated path that must exist after the
	// unpack, relative to the target directory.
	MustContain string `json:"mustContain"`
	// Strip is the number of leading root folders the archive wraps its
	// contents in, removed as it is unpacked.
	Strip int `json:"strip"`
}

// Installable is one entry of the shipped catalogue: a runtime Locafy can
// install for its kind and this machine's platform. Adding a runtime or a
// version is a data change, not code.
type Installable struct {
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Version string `json:"version"`
	Spec    Spec   `json:"spec"`
}

// nodejs builds the catalogue entries for one Node.js release, whose
// archives wrap everything in a single root folder.
func nodejs(version string) []Installable {
	base := "https://nodejs.org/dist/v" + version + "/node-v" + version
	specs := map[string]Spec{
		"windows/amd64": {
			Kind: "node", Version: version,
			URL: base + "-win-x64.zip", Format: "zip",
			MustContain: "node.exe", Strip: 1,
		},
		"darwin/arm64": {
			Kind: "node", Version: version,
			URL: base + "-darwin-arm64.tar.gz", Format: "targz",
			MustContain: "bin/node", Strip: 1,
		},
		"darwin/amd64": {
			Kind: "node", Version: version,
			URL: base + "-darwin-x64.tar.gz", Format: "targz",
			MustContain: "bin/node", Strip: 1,
		},
		"linux/amd64": {
			Kind: "node", Version: version,
			URL: base + "-linux-x64.tar.gz", Format: "targz",
			MustContain: "bin/node", Strip: 1,
		},
		"linux/arm64": {
			Kind: "node", Version: version,
			URL: base + "-linux-arm64.tar.gz", Format: "targz",
			MustContain: "bin/node", Strip: 1,
		},
	}
	spec, ok := specs[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return nil
	}
	return []Installable{{
		Kind:    "node",
		Label:   "Node.js " + version,
		Version: version,
		Spec:    spec,
	}}
}

// Catalogue lists everything this machine can install, newest version first
// per kind. It is the seam tests replace with a controlled entry instead of
// reaching for the network.
var Catalogue = func() []Installable {
	return nodejs("22.14.0")
}

// Find names the installable for a kind on this machine, if the catalogue
// has one.
func Find(kindID string) (Installable, bool) {
	for _, installable := range Catalogue() {
		if installable.Kind == kindID {
			return installable, true
		}
	}
	return Installable{}, false
}
