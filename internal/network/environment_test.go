package network

import (
	"strings"
	"testing"
)

func TestCertificateEnvironmentAddsTheTrustedCertificate(t *testing.T) {
	env := withCertificate([]string{"PATH=C:\\bin"}, "C:\\certs\\ca.crt")

	values := environmentMap(env)
	for _, name := range certificateVariables {
		if values[name] != "C:\\certs\\ca.crt" {
			t.Errorf("%s = %q, want the local certificate", name, values[name])
		}
	}
	if values["PATH"] != "C:\\bin" {
		t.Errorf("PATH = %q, want the inherited value kept", values["PATH"])
	}
	if _, ok := values["NODE_TLS_REJECT_UNAUTHORIZED"]; ok {
		t.Error("NODE_TLS_REJECT_UNAUTHORIZED was set; verification must never be disabled")
	}
}

func TestCertificateEnvironmentKeepsAnExistingOverride(t *testing.T) {
	env := withCertificate([]string{"NODE_EXTRA_CA_CERTS=C:\\other\\ca.crt"}, "C:\\certs\\ca.crt")

	values := environmentMap(env)
	if values["NODE_EXTRA_CA_CERTS"] != "C:\\other\\ca.crt" {
		t.Errorf("NODE_EXTRA_CA_CERTS = %q, want the application's own value kept", values["NODE_EXTRA_CA_CERTS"])
	}
	if values["SSL_CERT_FILE"] != "C:\\certs\\ca.crt" {
		t.Errorf("SSL_CERT_FILE = %q, want the local certificate", values["SSL_CERT_FILE"])
	}
}

func TestCertificateEnvironmentWithoutACertificateChangesNothing(t *testing.T) {
	env := withCertificate([]string{"PATH=C:\\bin"}, "")

	if len(env) != 1 || !strings.HasPrefix(env[0], "PATH=") {
		t.Fatalf("env = %v, want the environment unchanged", env)
	}
}

func environmentMap(env []string) map[string]string {
	values := make(map[string]string, len(env))
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			values[name] = value
		}
	}
	return values
}
