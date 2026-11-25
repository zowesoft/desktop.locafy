package services

import (
	"os"

	"locafy.dev/desktop/internal/network"
)

// serviceEnvironment is the machine environment plus Locafy's certificate.
//
// A service is a process Locafy starts, exactly like an application: an engine
// or a Node-based tool it runs may call an HTTPS local domain during setup or
// while it serves, and Node ignores the Windows trust store. The variables come
// from the same helper the application supervisor uses, so there is one answer
// to "how does a process Locafy starts trust Locafy's CA".
func serviceEnvironment() []string {
	return network.CertificateEnvironment(os.Environ())
}
