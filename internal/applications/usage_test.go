package applications

import (
	"errors"
	"testing"
	"time"
)

// fakeReadings replaces the process table: each pid answers with canned CPU
// seconds and resident memory.
func fakeReadings(t *testing.T, cores int, times map[int][2]float64, memory map[int]uint64, children map[int][]int) {
	t.Helper()

	previousCores, previousTimes, previousMemory, previousTree :=
		logicalCores, processTimes, processMemory, treePIDs
	logicalCores = func() (int, error) { return cores, nil }
	processTimes = func(pid int) (float64, float64, error) {
		if reading, known := times[pid]; known {
			return reading[0], reading[1], nil
		}
		return 0, 0, errors.New("no such process")
	}
	processMemory = func(pid int) (uint64, error) {
		if bytes, known := memory[pid]; known {
			return bytes, nil
		}
		return 0, errors.New("no such process")
	}
	treePIDs = func(root int) ([]int, error) {
		pids := append([]int{root}, children[root]...)
		return pids, nil
	}
	t.Cleanup(func() {
		logicalCores, processTimes, processMemory, treePIDs =
			previousCores, previousTimes, previousMemory, previousTree
	})
}

func TestUsageComputesTheDeltaBetweenSamples(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	app := projectApplication(t, database, project, "api", live)

	// A synthetic run with a canned tree: root 4242 with one child, 0.12 CPU
	// seconds of accumulated work, 12 MiB resident.
	fakeReadings(t, 2,
		map[int][2]float64{4242: {0.05, 0.03}, 4243: {0.02, 0.02}},
		map[int]uint64{4242: 10 << 20, 4243: 2 << 20},
		map[int][]int{4242: {4243}},
	)

	supervisor := NewSupervisor(database, nil)
	events := &runRecorder{}
	supervisor.emit = events.emit
	synthetic := &run{done: make(chan struct{})}
	supervisor.mu.Lock()
	supervisor.runs[app.ID] = synthetic
	supervisor.mu.Unlock()
	// The port and health scans are faked out, so the only wall time between
	// the two samples is the deliberate sleep.
	fakePorts(t, nil, func(pid int, root int) bool { return true })
	fakeHealth(t, func(port int) error { return nil })

	supervisor.scanRuntime(app.ID, synthetic, 4242)
	// CPU has no baseline on the first sample; memory is a direct reading.
	if usage, running := supervisor.usageOf(app.ID); running && usage.CPUPercent != 0 {
		t.Fatalf("first sample cpu = %v%%, want zero with no baseline", usage.CPUPercent)
	}

	time.Sleep(30 * time.Millisecond)
	// The tree worked 0.07 more CPU seconds since the baseline.
	fakeReadings(t, 2,
		map[int][2]float64{4242: {0.08, 0.04}, 4243: {0.04, 0.03}},
		map[int]uint64{4242: 11 << 20, 4243: 2 << 20},
		map[int][]int{4242: {4243}},
	)
	supervisor.scanRuntime(app.ID, synthetic, 4242)
	usage, running := supervisor.usageOf(app.ID)
	if !running {
		t.Fatal("no reading after the second sample")
	}
	// The exact wall time is not knowable in a test; assert the reading is a
	// plausible share of the work, on the machine scale.
	if usage.CPUPercent < 20 || usage.CPUPercent > 500 {
		t.Fatalf("cpu = %v%%, want a share of 0.07s over ~30ms on 2 cores", usage.CPUPercent)
	}
	if usage.MemoryBytes != 13<<20 {
		t.Fatalf("memory = %d, want the tree's resident sum", usage.MemoryBytes)
	}
}

func TestProjectUsageSumsRunningApplications(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	one := projectApplication(t, database, project, "api", live)
	two := projectApplication(t, database, project, "worker", live)
	projectApplication(t, database, project, "idle", "")

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(one.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, one.ID, StatusRunning)
	if _, err := supervisor.Start(two.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, two.ID, StatusRunning)
	t.Cleanup(supervisor.StopAll)

	fakeReadings(t, 4,
		map[int][2]float64{},
		map[int]uint64{},
		map[int][]int{},
	)
	// Seed the runs with canned readings, as a scan would have.
	supervisor.mu.Lock()
	supervisor.runs[one.ID].usage = Usage{CPUPercent: 12.5, MemoryBytes: 40 << 20}
	supervisor.runs[two.ID].usage = Usage{CPUPercent: 7.5, MemoryBytes: 8 << 20}
	supervisor.mu.Unlock()

	report, err := supervisor.ProjectUsage(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.RunningApplications != 2 {
		t.Fatalf("running = %d, want 2", report.RunningApplications)
	}
	if report.CPUPercent != 20 {
		t.Fatalf("cpu = %v%%, want 20", report.CPUPercent)
	}
	if report.MemoryBytes != 48<<20 {
		t.Fatalf("memory = %d, want 48 MiB", report.MemoryBytes)
	}
}

func TestProjectUsageOfAnIdleProjectIsZero(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	projectApplication(t, database, project, "api", live)

	report, err := NewSupervisor(database, nil).ProjectUsage(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.RunningApplications != 0 || report.CPUPercent != 0 || report.MemoryBytes != 0 {
		t.Fatalf("report = %+v, want zeros with nothing running", report)
	}
}

func TestUsageBaselineIsDroppedWhenTheProcessEnds(t *testing.T) {
	database := openDatabase(t)
	fastReady(t)
	project := newProject(t, database)

	live := quoteCommand(script(t, t.TempDir(), "live", 0, true))
	app := projectApplication(t, database, project, "api", live)

	supervisor := NewSupervisor(database, nil)
	if _, err := supervisor.Start(app.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, app.ID, StatusRunning)
	root := supervisor.runs[app.ID].pid

	supervisor.mu.Lock()
	supervisor.cpuBaseline[root] = cpuTimes{total: 1.5, at: time.Now()}
	supervisor.mu.Unlock()

	if err := supervisor.Stop(app.ID); err != nil {
		t.Fatal(err)
	}
	waitForStatus(t, database, app.ID, StatusStopped)

	supervisor.mu.Lock()
	_, kept := supervisor.cpuBaseline[root]
	supervisor.mu.Unlock()
	if kept {
		t.Fatal("the baseline outlived its process")
	}
}
