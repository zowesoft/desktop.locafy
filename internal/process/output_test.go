package process

import (
	"os"
	"sync"
	"testing"
	"time"
)

func TestStreamEndsWhenAChildKeepsTheWriterOpen(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { writer.Close() })

	var mu sync.Mutex
	lines := []string{}
	exited := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		Stream(reader, exited, func(line string) {
			mu.Lock()
			lines = append(lines, line)
			mu.Unlock()
		})
		close(finished)
	}()

	if _, err := writer.WriteString("last words\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	// The process exits, but the write end stays open: a child inherited it.
	close(exited)

	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("the stream is still reading a pipe its process left open")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 1 || lines[0] != "last words" {
		t.Fatalf("lines = %q, want the process's last line kept", lines)
	}
}

func TestStreamEndsAtEOFWithoutWaitingForTheQuietPeriod(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	var mu sync.Mutex
	lines := []string{}
	finished := make(chan struct{})

	go func() {
		Stream(reader, make(chan struct{}), func(line string) {
			mu.Lock()
			lines = append(lines, line)
			mu.Unlock()
		})
		close(finished)
	}()

	writer.WriteString("one\ntwo\n")
	writer.Close()

	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("the stream did not end at EOF")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 2 || lines[0] != "one" || lines[1] != "two" {
		t.Fatalf("lines = %q, want both complete lines", lines)
	}
}
