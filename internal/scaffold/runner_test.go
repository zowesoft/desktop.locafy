package scaffold

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stub writes a small executable that prints a line and creates the marker a
// recipe verifies, so runner tests never depend on a real toolchain.
func stub(t *testing.T, dir string, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "stub.cmd")
		content := "@echo off\r\necho stub output\r\n" + "echo done> \"" + marker + "\"\r\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "stub.sh")
	content := "#!/bin/sh\necho stub output\necho done > \"" + marker + "\"\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// cancelStub runs a child that outlives the top process, so a cancel that
// stops only the top process would leave the job waiting on output that never
// ends. The child records a heartbeat file: the test uses it to know the child
// started, and that it is gone afterwards. Both processes give up after five
// minutes so a failed test leaks nothing for long.
func cancelStub(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		tick := filepath.Join(dir, "tick.cmd")
		tickContent := "@echo off\r\n" +
			"setlocal enabledelayedexpansion\r\n" +
			"set /a n=0\r\n" +
			":loop\r\n" +
			"echo t>>child.alive\r\n" +
			"ping -n 2 127.0.0.1 >nul\r\n" +
			"set /a n+=1\r\n" +
			"if !n! lss 300 goto loop\r\n"
		if err := os.WriteFile(tick, []byte(tickContent), 0o644); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "cancel.cmd")
		content := "@echo off\r\n" +
			"start /b \"\" \"" + tick + "\"\r\n" +
			"ping -n 610 127.0.0.1 >nul\r\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "cancel.sh")
	content := "#!/bin/sh\n" +
		"( i=0; while [ $i -lt 300 ]; do echo t >> child.alive; sleep 1; i=$((i+1)); done ) &\n" +
		"wait\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunnerStreamsOutputAndSucceeds(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "app")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := stub(t, dir, filepath.Join(target, "marker.txt"))

	var events []string
	manager := NewManager(nil, "", func(name string, data any) {
		if name == "scaffold:output" {
			if line, ok := data.(map[string]any)["line"].(string); ok {
				events = append(events, line)
			}
		}
	})

	job, err := manager.Start(RunSpec{
		RecipeID:      "stub",
		Target:        target,
		CreatedTarget: false,
		Steps:         []Step{{Bin: bin, Args: []string{"one"}, Cwd: "parent", Display: "stub one"}},
		Verify:        []string{"marker.txt"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for {
		current, lines, err := manager.Get(job.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if current.Status == JobSucceeded {
			if !strings.Contains(strings.Join(lines, "\n"), "stub output") {
				t.Errorf("lines = %v, want the stub's output", lines)
			}
			break
		}
		if current.Status == JobFailed || current.Status == JobCancelled {
			t.Fatalf("job ended %s: %s", current.Status, current.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not finish; state %s", current.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !strings.Contains(strings.Join(events, "\n"), "stub output") {
		t.Errorf("events = %v, want streamed output", events)
	}
}

func TestRunnerReportsMissingFilesAndFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "app")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := stub(t, dir, filepath.Join(target, "marker.txt"))

	manager := NewManager(nil, "", nil)
	job, err := manager.Start(RunSpec{
		RecipeID: "stub",
		Target:   target,
		Steps:    []Step{{Bin: bin, Cwd: "parent", Display: "stub"}},
		Verify:   []string{"missing.txt"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForStatus(t, manager, job.ID, JobFailed)

	broken, err := manager.Start(RunSpec{
		RecipeID: "broken",
		Target:   target,
		Steps:    []Step{{Bin: filepath.Join(dir, "does-not-exist.exe"), Cwd: "parent", Display: "missing"}},
		Verify:   []string{"marker.txt"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForStatus(t, manager, broken.ID, JobFailed)
}

func TestCancelStopsTheStepAndItsChildren(t *testing.T) {
	dir := t.TempDir()
	bin := cancelStub(t, dir)
	heartbeat := filepath.Join(dir, "child.alive")

	manager := NewManager(nil, "", nil)
	job, err := manager.Start(RunSpec{
		RecipeID: "stub",
		Target:   filepath.Join(dir, "app"),
		Steps:    []Step{{Bin: bin, Cwd: "parent", Display: "stub"}},
		Verify:   []string{"marker.txt"},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Cancel only once the spawned child is writing, so the cancel lands on a
	// live tree rather than before the child exists.
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(heartbeat); err == nil {
			break
		}
		current, _, err := manager.Get(job.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if current.Status == JobFailed {
			t.Fatalf("job failed before the child started: %s", current.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stub's child never started; state %s", current.Status)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := manager.Cancel(job.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitForStatus(t, manager, job.ID, JobCancelled)

	// The child appends a heartbeat every second: if the cancel left it alive,
	// the file keeps growing.
	time.Sleep(time.Second)
	size := heartbeatSize(t, heartbeat)
	time.Sleep(2500 * time.Millisecond)
	if grown := heartbeatSize(t, heartbeat) - size; grown > 0 {
		t.Errorf("the child survived the cancel: heartbeat grew by %d bytes", grown)
	}
}

func heartbeatSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat heartbeat: %v", err)
	}
	return info.Size()
}

func TestDiscardOnlyRemovesAFoldersOwnScaffold(t *testing.T) {
	created := filepath.Join(t.TempDir(), "failed-app")
	if err := os.Mkdir(created, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Discard(Job{Status: JobFailed, CreatedTarget: true, TargetPath: created}); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatal("the failed folder was not removed")
	}

	existing := t.TempDir()
	if err := Discard(Job{Status: JobFailed, CreatedTarget: false, TargetPath: existing}); err == nil {
		t.Fatal("a pre-existing folder was removed")
	}
	if err := Discard(Job{Status: JobSucceeded, CreatedTarget: true, TargetPath: t.TempDir()}); err == nil {
		t.Fatal("a successful scaffold was removed")
	}
}

func waitForStatus(t *testing.T, manager *Manager, id string, want string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		job, _, err := manager.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if job.Status == want {
			return
		}
		if job.Status == JobSucceeded || job.Status == JobCancelled {
			t.Fatalf("job ended %s, want %s (%s)", job.Status, want, job.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("job stayed %s", job.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
