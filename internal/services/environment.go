package services

import (
	"os"

	"locafy.dev/desktop/internal/network"
)

// certificateEnvironment adds Locafy's CA to a started process's environment. It
// is a variable so a test can point it at a temporary certificate directory
// instead of writing a certificate into the user's data.
var certificateEnvironment = network.CertificateEnvironment

// serviceEnvironment is the machine environment plus Locafy's certificate.
//
// A service is a process Locafy starts, exactly like an application: an engine
// or a Node-based tool it runs may call an HTTPS local domain during setup or
// while it serves, and Node ignores the Windows trust store. The variables come
// from the same helper the application supervisor uses, so there is one answer
// to "how does a process Locafy starts trust Locafy's CA".
func serviceEnvironment() []string {
	return certificateEnvironment(os.Environ())
}
