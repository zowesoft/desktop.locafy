package applications

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// Health states an application can report. Empty means unknown: not running,
// not bound to a port yet, or not checked since it started.
const (
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
)

// healthTimeout bounds one connection attempt. A server that cannot accept a
// connection in this window is not answering.
const healthTimeout = 1500 * time.Millisecond

// healthProbe is a seam: tests answer without opening a socket.
var healthProbe = probeHealth

// probeHealth reports whether anything accepts a connection on the port. It
// dials, closes and writes nothing, so checking does not drive the application.
func probeHealth(port int) error {
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	connection, err := net.DialTimeout("tcp", address, healthTimeout)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", address, err)
	}
	return connection.Close()
}
