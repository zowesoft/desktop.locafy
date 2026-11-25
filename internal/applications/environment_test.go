package applications

import (
	"os"
	"testing"

	"locafy.dev/desktop/internal/network"
)

// TestMain points the certificate environment at a temporary directory for the
// whole suite. Starting an application must not create a certificate authority
// in the user's data directory just because a test ran.
func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "locafy-certificates")
	if err != nil {
		panic("create temporary certificate directory: " + err.Error())
	}

	previous := certificateEnvironment
	certificateEnvironment = func(env []string) []string {
		return network.CertificateEnvironmentAt(env, directory)
	}

	code := m.Run()

	certificateEnvironment = previous
	_ = os.RemoveAll(directory)
	os.Exit(code)
}
