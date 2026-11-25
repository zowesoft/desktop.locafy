package network

import (
	"os"
	"strings"
)

// CertificateEnvironment adds Locafy's CA to the variables that runtimes read
// for extra trust anchors, and returns env unchanged when the certificate
// cannot be found.
//
// This exists because the Windows trust store is not the only store that
// matters. Node ships its own Mozilla bundle and ignores the system store, so a
// server-side fetch from a Next.js dev server to https://api.project.test fails
// with UNABLE_TO_VERIFY_LEAF_SIGNATURE even after the user has trusted the
// certificate in Windows. Applications Locafy starts therefore receive the CA
// through the environment rather than by editing their scripts.
//
// An application that already sets one of these variables keeps its value: an
// application may deliberately trust a CA Locafy knows nothing about.
func CertificateEnvironment(env []string) []string {
	path := CertificateFile()
	if path == "" {
		return env
	}
	return withCertificate(env, path)
}

// CertificateFile makes sure the local authority exists and returns the
// certificate other programs can trust, or "" when it cannot be created.
func CertificateFile() string {
	authority := NewAuthority(CertificatesDirectory())
	if _, err := authority.Ensure(); err != nil {
		return ""
	}
	if _, err := os.Stat(authority.CAPath()); err != nil {
		return ""
	}
	return authority.CAPath()
}

// certificateVariables are read by the runtimes that do not consult the system
// store: Node, Python's ssl and requests, and curl. Go and .NET read the system
// store, and browsers are covered by the trust step in the interface.
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

	for _, name := range certificateVariables {
		if present[name] {
			continue
		}
		env = append(env, name+"="+path)
	}
	return env
}
