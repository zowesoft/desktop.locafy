// Package logs provides the engine's log file writer with size-based
// rotation.
package logs

import (
	"fmt"
	"os"
	"sync"
)

// Rotator writes to a log file, rolling it over to numbered backups when
// it grows past a size threshold. At most backups files are kept.
type Rotator struct {
	path     string
	maxBytes int64
	backups  int

	mu   sync.Mutex
	file *os.File
	size int64
}

// New opens (or creates) the log file at path. maxBytes is the size at
// which the file rolls over; backups is how many rolled-over files are
// kept.
func New(path string, maxBytes int64, backups int) (*Rotator, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("max bytes must be positive, got %d", maxBytes)
	}
	if backups <= 0 {
		return nil, fmt.Errorf("backups must be positive, got %d", backups)
	}
	rotator := &Rotator{path: path, maxBytes: maxBytes, backups: backups}
	if err := rotator.open(); err != nil {
		return nil, err
	}
	return rotator, nil
}

// open seeks to the end of the log file so appends continue where the
// last run stopped.
func (r *Rotator) open() error {
	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("stat log file: %w", err)
	}
	r.file = file
	r.size = info.Size()
	return nil
}

// Write appends a record, rolling the file over first when the record
// would push it past the size threshold.
func (r *Rotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size > 0 && r.size+int64(len(p)) > r.maxBytes {
		if err := r.roll(); err != nil {
			return 0, err
		}
	}
	n, err := r.file.Write(p)
	r.size += int64(n)
	if err != nil {
		return n, fmt.Errorf("write log record: %w", err)
	}
	return n, nil
}

// roll closes the current file, shifts backups, and reopens a fresh one.
// The oldest backup is dropped to keep the bounded count.
func (r *Rotator) roll() error {
	if err := r.file.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	r.file = nil

	if err := os.Remove(backupPath(r.path, r.backups)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove oldest log backup: %w", err)
	}
	for i := r.backups - 1; i >= 1; i-- {
		if err := os.Rename(backupPath(r.path, i), backupPath(r.path, i+1)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("shift log backup %d: %w", i, err)
		}
	}
	if err := os.Rename(r.path, backupPath(r.path, 1)); err != nil {
		return fmt.Errorf("rotate log file: %w", err)
	}
	return r.open()
}

func backupPath(path string, n int) string {
	return fmt.Sprintf("%s.%d", path, n)
}

// Close flushes and closes the underlying file.
func (r *Rotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}
