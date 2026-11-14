package monitor

import (
	"testing"
	"time"
)

func TestSnapshotReportsMemoryAndDisk(t *testing.T) {
	sampler := NewSampler(t.TempDir())
	defer sampler.Close()

	snapshot := sampler.Snapshot()

	if snapshot.Memory.TotalBytes == 0 {
		t.Fatal("memory total is zero, expected the machine's installed memory")
	}
	if snapshot.Memory.UsedBytes > snapshot.Memory.TotalBytes {
		t.Errorf("memory used %d exceeds total %d", snapshot.Memory.UsedBytes, snapshot.Memory.TotalBytes)
	}
	if snapshot.Disk.TotalBytes == 0 {
		t.Fatal("disk total is zero, expected the volume holding the sampled path")
	}
	if snapshot.Disk.UsedBytes > snapshot.Disk.TotalBytes {
		t.Errorf("disk used %d exceeds total %d", snapshot.Disk.UsedBytes, snapshot.Disk.TotalBytes)
	}
	if snapshot.CapturedAt.IsZero() {
		t.Error("captured time is zero")
	}
	if elapsed := time.Since(snapshot.CapturedAt); elapsed > time.Minute {
		t.Errorf("captured time is %s old, expected a recent reading", elapsed)
	}
}

func TestPercentagesStayInRange(t *testing.T) {
	sampler := NewSampler(t.TempDir())
	defer sampler.Close()

	snapshot := sampler.Snapshot()

	for name, value := range map[string]float64{
		"cpu":    snapshot.CPUPercent,
		"memory": snapshot.Memory.Percent,
		"disk":   snapshot.Disk.Percent,
	} {
		if value < 0 || value > 100 {
			t.Errorf("%s percentage is %v, expected 0-100", name, value)
		}
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	sampler := NewSampler(t.TempDir())

	sampler.Close()
	sampler.Close()
}

func TestMissingPathLeavesDiskEmpty(t *testing.T) {
	sampler := NewSampler(t.TempDir() + "/does-not-exist")
	defer sampler.Close()

	snapshot := sampler.Snapshot()

	if snapshot.Disk.TotalBytes != 0 {
		t.Errorf("disk total is %d for a missing path, expected no reading", snapshot.Disk.TotalBytes)
	}
	if snapshot.Memory.TotalBytes == 0 {
		t.Error("memory reading was lost when the disk path was invalid")
	}
}
