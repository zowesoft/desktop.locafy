// Package monitor samples machine resource usage so the desktop chrome can
// report what the machine is doing, not only whether the engine is running.
package monitor

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// refreshInterval is how often the background sampler re-reads the machine.
// CPU usage is a delta since the previous reading, so the interval is also the
// window that percentage is averaged over.
const refreshInterval = 2 * time.Second

// Usage describes one measured resource in bytes and as a percentage.
type Usage struct {
	UsedBytes  uint64
	TotalBytes uint64
	Percent    float64
}

// Snapshot is one reading of machine usage.
type Snapshot struct {
	CPUPercent float64
	Memory     Usage
	Disk       Usage
	CapturedAt time.Time
}

// Sampler keeps a recent reading of machine usage so callers can read it
// without blocking on the measurement itself.
type Sampler struct {
	diskPath string

	mu       sync.RWMutex
	snapshot Snapshot

	stop    chan struct{}
	stopped sync.Once
}

// NewSampler starts sampling the machine and measures the volume holding
// diskPath. A sampler is cheap to read and lives for the life of the process;
// callers should Close it to release the background goroutine.
func NewSampler(diskPath string) *Sampler {
	sampler := &Sampler{diskPath: diskPath, stop: make(chan struct{})}

	// CPU usage is reported as a delta since the previous call, so prime the
	// baseline here and let the first tick produce a real percentage.
	_, _ = cpu.Percent(0, false)
	sampler.snapshot = sampler.measure()

	go sampler.run()

	return sampler
}

// Snapshot returns the most recent reading.
func (s *Sampler) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.snapshot
}

// Close stops the background sampler. It is safe to call more than once.
func (s *Sampler) Close() {
	s.stopped.Do(func() { close(s.stop) })
}

func (s *Sampler) run() {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			reading := s.measure()
			s.mu.Lock()
			s.snapshot = reading
			s.mu.Unlock()
		}
	}
}

// measure reads each resource independently: a failure on one leaves that
// field zero rather than discarding the whole reading, because a machine that
// cannot report its disk should still report its memory.
func (s *Sampler) measure() Snapshot {
	snapshot := Snapshot{CapturedAt: time.Now().UTC()}

	if percentages, err := cpu.Percent(0, false); err == nil && len(percentages) > 0 {
		snapshot.CPUPercent = round1(percentages[0])
	}

	if stats, err := mem.VirtualMemory(); err == nil {
		snapshot.Memory = Usage{
			UsedBytes:  stats.Used,
			TotalBytes: stats.Total,
			Percent:    round1(stats.UsedPercent),
		}
	}

	if stats, err := disk.Usage(s.diskPath); err == nil {
		snapshot.Disk = Usage{
			UsedBytes:  stats.Used,
			TotalBytes: stats.Total,
			Percent:    round1(stats.UsedPercent),
		}
	}

	return snapshot
}

func round1(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}
