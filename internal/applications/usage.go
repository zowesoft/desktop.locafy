package applications

import (
	"math"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
)

// round1 keeps one decimal, the precision the machine meter reports at.
func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

// Usage is what one live application's process tree is consuming right now.
// CPU is normalized to the whole machine (100% = every core), matching the
// machine meter's scale.
type Usage struct {
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryBytes uint64  `json:"memoryBytes"`
}

// ProjectUsage aggregates the usage of a project's running applications.
// Nothing running reports zeros, not a failure.
type ProjectUsage struct {
	CPUPercent          float64 `json:"cpuPercent"`
	MemoryBytes         uint64  `json:"memoryBytes"`
	RunningApplications int     `json:"runningApplications"`
}

// EventUsage announces a changed per-application reading.
const EventUsage = "application:usage"

// cpuTimes is one sample of a pid's accumulated CPU seconds.
type cpuTimes struct {
	total float64
	at    time.Time
}

// Seams for tests: canned readings instead of a live process table.
var (
	// logicalCores normalizes a tree's CPU seconds to the machine scale.
	logicalCores = func() (int, error) { return cpu.Counts(true) }
	// processTimes reads a pid's accumulated user and system seconds.
	processTimes = func(pid int) (user float64, system float64, err error) {
		p, err := process.NewProcess(int32(pid))
		if err != nil {
			return 0, 0, err
		}
		times, err := p.Times()
		if err != nil {
			return 0, 0, err
		}
		return times.User, times.System, nil
	}
	// processMemory reads a pid's resident memory.
	processMemory = func(pid int) (uint64, error) {
		p, err := process.NewProcess(int32(pid))
		if err != nil {
			return 0, err
		}
		memory, err := p.MemoryInfo()
		if err != nil {
			return 0, err
		}
		return memory.RSS, nil
	}
	// treePIDs lists a root pid and everything below it.
	treePIDs = func(root int) ([]int, error) {
		pids := []int{root}
		p, err := process.NewProcess(int32(root))
		if err != nil {
			return pids, nil
		}
		children, err := p.Children()
		if err != nil {
			return pids, nil
		}
		for _, child := range children {
			pids = append(pids, int(child.Pid))
		}
		return pids, nil
	}
)

// recordUsage measures the tree's CPU delta and resident memory for one tick
// and announces a changed reading. The first sample after a start has no
// baseline and reports zero rather than a guess.
func (s *Supervisor) recordUsage(id string, live *run, root int) {
	if exited(live) {
		return
	}
	cores, err := logicalCores()
	if err != nil || cores <= 0 {
		return
	}

	pids, err := treePIDs(root)
	if err != nil {
		return
	}
	now := time.Now()
	total := 0.0
	memory := uint64(0)
	for _, pid := range pids {
		if user, system, err := processTimes(pid); err == nil {
			total += user + system
		}
		if bytes, err := processMemory(pid); err == nil {
			memory += bytes
		}
	}

	// Commit the baseline only while the run is live, in the same lock that
	// finalises it: a scan already in flight must not restore a baseline that a
	// Stop has just dropped unless its reading was taken before the stop.
	s.mu.Lock()
	if live.finalised {
		s.mu.Unlock()
		return
	}
	if s.cpuBaseline == nil {
		s.cpuBaseline = map[int]cpuTimes{}
	}
	var percent float64
	if previous, seen := s.cpuBaseline[root]; seen {
		wall := now.Sub(previous.at).Seconds()
		if wall > 0 {
			percent = round1((total - previous.total) / wall / float64(cores) * 100)
		}
	}
	s.cpuBaseline[root] = cpuTimes{total: total, at: now}
	// Baselines of pids that left the tree are dropped on the next pass
	// through the running set, not here: the map is keyed by root.

	usage := Usage{CPUPercent: percent, MemoryBytes: memory}
	changed := live.usage != usage
	if changed {
		live.usage = usage
	}
	s.mu.Unlock()
	if !changed {
		return
	}
	if s.emit != nil {
		s.emit(EventUsage, map[string]any{
			"applicationId": id,
			"cpuPercent":    usage.CPUPercent,
			"memoryBytes":   usage.MemoryBytes,
		})
	}
}

// dropBaseline forgets a tree's CPU baseline when its process is gone, so a
// recycled pid never inherits the previous tenant's delta.
func (s *Supervisor) dropBaseline(root int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cpuBaseline, root)
}

// usageOf returns the last measured usage of a running application.
func (s *Supervisor) usageOf(id string) (Usage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	live, ok := s.runs[id]
	if !ok || live.usage == (Usage{}) {
		return Usage{}, false
	}
	return live.usage, true
}

// ProjectUsage sums the usage of a project's running applications.
func (s *Supervisor) ProjectUsage(projectID string) (*ProjectUsage, error) {
	apps, err := List(s.database, projectID)
	if err != nil {
		return nil, err
	}
	report := &ProjectUsage{}
	for _, app := range apps {
		if usage, running := s.usageOf(app.ID); running {
			report.CPUPercent = round1(report.CPUPercent + usage.CPUPercent)
			report.MemoryBytes += usage.MemoryBytes
			report.RunningApplications++
		}
	}
	return report, nil
}
