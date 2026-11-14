package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatorRollsAndKeepsBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locafy.log")

	rotator, err := New(path, 100, 2)
	if err != nil {
		t.Fatalf("new rotator: %v", err)
	}

	const records = 12
	for i := 0; i < records; i++ {
		record := fmt.Sprintf("record-%02d %s\n", i, strings.Repeat("x", 20))
		if _, err := rotator.Write([]byte(record)); err != nil {
			t.Fatalf("write record %d: %v", i, err)
		}
	}
	if err := rotator.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// The active file holds the newest records; the two backups hold the
	// older generations in order; everything older is gone.
	main, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read active log: %v", err)
	}
	backup1, err := os.ReadFile(backupPath(path, 1))
	if err != nil {
		t.Fatalf("read backup 1: %v", err)
	}
	backup2, err := os.ReadFile(backupPath(path, 2))
	if err != nil {
		t.Fatalf("read backup 2: %v", err)
	}
	if _, err := os.Stat(backupPath(path, 3)); !os.IsNotExist(err) {
		t.Fatalf("third backup exists beyond the kept count: %v", err)
	}

	if !strings.Contains(string(main), "record-11") {
		t.Fatalf("active log = %q, want the newest record", main)
	}
	if !strings.Contains(string(backup1), "record-08") || strings.Contains(string(backup1), "record-11") {
		t.Fatalf("backup 1 = %q, want the records before the active file", backup1)
	}
	if !strings.Contains(string(backup2), "record-05") || strings.Contains(string(backup2), "record-06") {
		t.Fatalf("backup 2 = %q, want the next-older generation", backup2)
	}

	var seen int
	for _, content := range []string{string(main), string(backup1), string(backup2)} {
		seen += strings.Count(content, "record-")
	}
	// Three records fit per file, so the bounded two-backup window holds
	// nine records and the single oldest generation is dropped.
	if seen != 9 {
		t.Fatalf("records across log files = %d, want 9", seen)
	}
}

func TestRotatorAppendsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locafy.log")

	first, err := New(path, 1024, 2)
	if err != nil {
		t.Fatalf("new rotator: %v", err)
	}
	if _, err := first.Write([]byte("first-run\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := New(path, 1024, 2)
	if err != nil {
		t.Fatalf("reopen rotator: %v", err)
	}
	if _, err := second.Write([]byte("second-run\n")); err != nil {
		t.Fatalf("write after reopen: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(content) != "first-run\nsecond-run\n" {
		t.Fatalf("log = %q, want both runs appended", content)
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locafy.log")

	if _, err := New(path, 0, 2); err == nil {
		t.Fatal("zero max bytes accepted, want error")
	}
	if _, err := New(path, 1024, 0); err == nil {
		t.Fatal("zero backups accepted, want error")
	}
}
