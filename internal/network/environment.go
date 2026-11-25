package network

import (
	"os"
	"strings"
	"sync"
)

// CertificateEnvironment adds Locafy's CA to the variables that runtimes read
// for extra trust anchors, reading the certificate from the user's data
// directory, and returns env unchanged when it cannot be found.
//
// This exists because the Windows trust store is not the only store that
// matters. Node ships its own Mozilla bundle and ignores the system store, so a
// server-side fetch from a Next.js dev server to https://api.project.test fails
// with UNABLE_TO_VERIFY_LEAF_SIGNATURE even after the user has trusted the
// certificate in Windows. Applications and services Locafy starts therefore
// receive the CA through the environment rather than by editing their scripts.
func CertificateEnvironment(env []string) []string {
	return CertificateEnvironmentAt(env, CertificatesDirectory())
}

// CertificateEnvironmentAt is CertificateEnvironment for a named certificate
// directory. It exists so a test can use a temporary directory instead of
// creating a certificate in the user's data.
func CertificateEnvironmentAt(env []string, directory string) []string {
	path := certificateFileAt(directory)
	if path == "" {
		return env
	}
	return withCertificate(env, path)
}

// CertificateFile makes sure the local authority exists and returns the
// certificate other programs can trust, or "" when it cannot be created.
func CertificateFile() string {
	return certificateFileAt(CertificatesDirectory())
}

// certificatePaths remembers the certificate each directory resolved to. Every
// started process asks for the environment, and re-reading and parsing the CA on
// each spawn is work with one stable answer: the path never changes while the
// file lives where the authority puts it. An unavailable directory is not cached,
// so a later start can still create the certificate.
var certificatePaths sync.Map

func certificateFileAt(directory string) string {
	if path, ok := certificatePaths.Load(directory); ok {
		return path.(string)
	}

	authority := NewAuthority(directory)
	if _, err := authority.Ensure(); err != nil {
		return ""
	}
	if _, err := os.Stat(authority.CAPath()); err != nil {
		return ""
	}
	certificatePaths.Store(directory, authority.CAPath())
	return authority.CAPath()
}

// An application that already sets one of these variables keeps its value: it
// may deliberately trust a CA Locafy knows nothing about.
var certificateVariables = []string{
	"NODE_EXTRA_CA_CERTS",
	"SSL_CERT_FILE",
	"REQUESTS_CA_BUNDLE",
	"CURL_CA_BUNDLE",
}

func withCertificate(env []string, path string) []string {
	if path == "" {
		return env
	}

	present := make(map[string]bool, len(env))
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if ok {
			present[name] = true
		}
	}

	// certificateVariables are read by the runtimes that do not consult the
	// system store: Node, Python's ssl and requests, and curl. Go and .NET read
	// the system store, and browsers are covered by the trust step in the
	// interface.
	for _, name := range certificateVariables {
		if present[name] {
			continue
		}
		env = append(env, name+"="+path)
	}
	return env
}
