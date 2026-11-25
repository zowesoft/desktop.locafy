package process

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// DrainQuiet is how long a stream must stay silent after its process has exited
// before it is cut. A child that outlived the process can hold the pipe open
// indefinitely; what the process itself wrote is already in the pipe and is
// read before the quiet period, so cutting the stream here keeps its last lines.
const DrainQuiet = 200 * time.Millisecond

// maxLineChunk bounds how much of one line is held in memory. A line longer than
// this is emitted in pieces rather than buffered whole, so a process that never
// writes a newline cannot grow the reader without limit.
const maxLineChunk = 64 * 1024

// Outputs owns the pipes a command writes its stdout and stderr to. They are
// Locafy's pipes rather than exec's, because exec's own Wait closes its pipes as
// soon as the process exits, which would discard output still waiting to be read.
type Outputs struct {
	Stdout *os.File
	Stderr *os.File

	writers []*os.File
}

// NewOutputs creates both pipes and points the command at their write ends.
// Release must follow a successful Start; Close is for a command that never ran.
func NewOutputs(command *exec.Cmd) (*Outputs, error) {
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		stdoutRead.Close()
		stdoutWrite.Close()
		return nil, err
	}

	command.Stdout = stdoutWrite
	command.Stderr = stderrWrite

	return &Outputs{
		Stdout:  stdoutRead,
		Stderr:  stderrRead,
		writers: []*os.File{stdoutWrite, stderrWrite},
	}, nil
}

// Release closes the parent's copies of the write ends once the child holds
// them, so the read ends reach EOF when every process holding them has gone.
func (o *Outputs) Release() {
	for _, writer := range o.writers {
		writer.Close()
	}
	o.writers = nil
}

// Close discards both ends, for a command that never started.
func (o *Outputs) Close() {
	o.Release()
	o.Stdout.Close()
	o.Stderr.Close()
}

// Stream reads a command's output line by line until the stream ends. It also
// ends when the process has exited and the stream has gone quiet without
// reaching EOF: a child that inherited the pipe would otherwise keep the reader
// blocked for as long as that child lives. Everything the process wrote before
// exiting is read first, so its last lines are not lost.
func Stream(reader io.ReadCloser, exited <-chan struct{}, emit func(string)) {
	var lastData atomic.Int64
	lastData.Store(time.Now().UnixNano())

	done := make(chan struct{})
	defer close(done)

	// The watchdog only arms once the process has exited. Until then a silent
	// stream is normal.
	go func() {
		select {
		case <-exited:
		case <-done:
			return
		}

		poll := DrainQuiet / 4
		for {
			time.Sleep(poll)
			select {
			case <-done:
				return
			default:
			}
			if time.Since(time.Unix(0, lastData.Load())) >= DrainQuiet {
				reader.Close()
				return
			}
		}
	}()

	buffered := bufio.NewReaderSize(&countingReader{reader: reader, last: &lastData}, maxLineChunk)
	for {
		chunk, err := buffered.ReadSlice('\n')
		text := strings.TrimRight(string(chunk), "\r\n")
		switch {
		case err == bufio.ErrBufferFull:
			if text != "" {
				emit(text)
			}
			continue
		case err != nil:
			if text != "" {
				emit(text)
			}
			return
		default:
			emit(text)
		}
	}
}

// countingReader records when data last arrived, so the watchdog can tell a
// stream that is still delivering from one an orphan has stalled.
type countingReader struct {
	reader io.Reader
	last   *atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	if n > 0 {
		c.last.Store(time.Now().UnixNano())
	}
	return n, err
}
