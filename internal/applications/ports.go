package applications

import (
	"fmt"
	"sort"

	gopsutilnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// listener is one TCP port a process is listening on.
type listener struct {
	Port    int
	PID     int
	Process string
}

// The two seams below keep port ownership testable without opening sockets or
// depending on which other processes this machine happens to run.
var (
	listeningFor = scanListeningPorts
	ownedBy      = processOwnedBy
)

// scanListeningPorts lists the machine's listening TCP ports with their owners.
// A pid of zero is skipped: the platform could not attribute the socket, and an
// unattributed port is not evidence of any application's ownership.
func scanListeningPorts() ([]listener, error) {
	connections, err := gopsutilnet.Connections("tcp")
	if err != nil {
		return nil, fmt.Errorf("list listening ports: %w", err)
	}

	names := map[int32]string{}
	listeners := make([]listener, 0, len(connections))
	for _, connection := range connections {
		if connection.Status != "LISTEN" || connection.Laddr.Port == 0 || connection.Pid <= 0 {
			continue
		}
		name, seen := names[connection.Pid]
		if !seen {
			if process, err := process.NewProcess(connection.Pid); err == nil {
				if value, err := process.Name(); err == nil {
					name = value
				}
			}
			names[connection.Pid] = name
		}
		listeners = append(listeners, listener{
			Port:    int(connection.Laddr.Port),
			PID:     int(connection.Pid),
			Process: name,
		})
	}
	return listeners, nil
}

// processOwnedBy reports whether pid is root or one of its descendants. The walk
// is bounded so a corrupt parent chain cannot loop.
func processOwnedBy(pid int, root int) bool {
	if pid <= 0 || root <= 0 {
		return false
	}
	current := int32(pid)
	for depth := 0; depth < 32; depth++ {
		if int(current) == root {
			return true
		}
		if current <= 1 {
			return false
		}
		owner, err := process.NewProcess(current)
		if err != nil {
			return false
		}
		parent, err := owner.Ppid()
		if err != nil || parent <= 0 {
			return false
		}
		current = parent
	}
	return false
}

// listenerOn returns the listener holding port, if one is recorded.
func listenerOn(port int, listeners []listener) (listener, bool) {
	for _, entry := range listeners {
		if entry.Port == port {
			return entry, true
		}
	}
	return listener{}, false
}

// portsOwnedBy returns every listening port of the process tree, lowest first.
func portsOwnedBy(listeners []listener, root int) []int {
	seenPort := map[int]bool{}
	// Descent is walked once per process, not once per socket it opened.
	ownership := map[int]bool{}
	ports := make([]int, 0, len(listeners))
	for _, entry := range listeners {
		if seenPort[entry.Port] {
			continue
		}
		owned, checked := ownership[entry.PID]
		if !checked {
			owned = ownedBy(entry.PID, root)
			ownership[entry.PID] = owned
		}
		if !owned {
			continue
		}
		seenPort[entry.Port] = true
		ports = append(ports, entry.Port)
	}
	sort.Ints(ports)
	return ports
}
