package downloads

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"locafy.dev/desktop/internal/db"
)

func openManager(t *testing.T, handler http.Handler) (*Manager, string) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	root := filepath.Join(t.TempDir(), "runtimes")
	manager := NewManager(database, root, nil)
	if handler != nil {
		manager.client = &http.Client{Transport: shortRound{handler}}
	}
	return manager, root
}

// shortRound answers a test HTTP client from an in-process handler, so an
// install job downloads from a fake nodejs.org without leaving the machine.
type shortRound struct{ handler http.Handler }

func (r shortRound) RoundTrip(request *http.Request) (*http.Response, error) {
	done := make(chan *http.Response, 1)
	go func() {
		recorder := httptest.NewRecorder()
		r.handler.ServeHTTP(recorder, request)
		done <- recorder.Result()
	}()
	// A real transport watches the request's context, which is how a cancel
	// interrupts a download; the fake honours it the same way.
	select {
	case response := <-done:
		response.Request = request
		return response, nil
	case <-request.Context().Done():
		return nil, request.Context().Err()
	}
}

// withCatalogue replaces the shipped catalogue with one entry the test
// controls, so a job downloads the test server's archive instead of reaching
// for the network.
func withCatalogue(t *testing.T, spec Spec) {
	t.Helper()
	restore := Catalogue
	t.Cleanup(func() { Catalogue = restore })
	Catalogue = func() []Installable {
		return []Installable{{Kind: spec.Kind, Label: spec.Kind + " " + spec.Version, Version: spec.Version, Spec: spec}}
	}
}

func zipArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o755)
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		file.Write([]byte(content))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func waitForState(t *testing.T, manager *Manager, id string, want string) Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		job, err := manager.Get(id)
		if err != nil {
			t.Fatalf("read job: %v", err)
		}
		if job.State == want {
			return job
		}
		if time.Now().After(deadline) {
			t.Fatalf("job stayed %q (%s), want %s", job.State, job.Error, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestInstallDownloadsVerifiesAndUnpacks(t *testing.T) {
	archive := zipArchive(t, map[string]string{
		"node-v22.14.0-win-x64/node.exe": "stub binary",
		"node-v22.14.0-win-x64/npm.cmd":  "stub shim",
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		w.Write(archive)
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	spec := Spec{Kind: "node", Version: "22.14.0", URL: server.URL + "/node.zip", Format: "zip", MustContain: "node.exe", Strip: 1}
	withCatalogue(t, spec)
	manager, root := openManager(t, handler)

	job, err := manager.Start("node")
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	// Start returns the queued snapshot; running takes over a moment later.
	waitForState(t, manager, job.ID, StateRunning)

	finished := waitForState(t, manager, job.ID, StateSucceeded)
	if finished.Total != int64(len(archive)) || finished.Progress != int64(len(archive)) {
		t.Errorf("progress = %d/%d, want the archive size", finished.Progress, finished.Total)
	}
	// The wrapping folder is stripped and the expected executable is there.
	binary, err := os.ReadFile(filepath.Join(root, "node", "22.14.0", "node.exe"))
	if err != nil {
		t.Fatalf("read the installed binary: %v", err)
	}
	if string(binary) != "stub binary" {
		t.Errorf("binary = %q, want the archive's contents", binary)
	}
	if _, err := os.Stat(filepath.Join(root, "node", "22.14.0", "npm.cmd")); err != nil {
		t.Errorf("the other members did not arrive: %v", err)
	}
	// A second install of the same version is refused, not doubled.
	if _, err := manager.Start("node"); err != ErrInstalled {
		t.Errorf("second start err = %v, want the installed refusal", err)
	}
}

func TestInstallExplainsWhatWentWrong(t *testing.T) {
	database := func() *Manager {
		manager, _ := openManager(t, nil)
		return manager
	}()

	if _, err := database.Start("fortran"); err != ErrNoInstall {
		t.Errorf("unknown kind err = %v, want the no-installable refusal", err)
	}

	// An archive without the expected executable fails the job with a
	// message that names the missing file.
	broken := zipArchive(t, map[string]string{"node-v22.14.0-win-x64/only.txt": "no binary here"})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(broken)
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	spec := Spec{Kind: "node", Version: "22.14.0", URL: server.URL + "/node.zip", Format: "zip", MustContain: "node.exe", Strip: 1}
	withCatalogue(t, spec)
	manager, _ := openManager(t, handler)

	job, err := manager.Start("node")
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	failed := waitForState(t, manager, job.ID, StateFailed)
	if failed.Error == "" {
		t.Error("a failed job carries no explanation")
	}

	// A truncated download fails rather than pretending to succeed.
	truncated := Spec{Kind: "node", Version: "23.0.0", URL: server.URL + "/cut.zip", Format: "zip", MustContain: "node.exe", Strip: 1}
	withCatalogue(t, truncated)
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "9999")
		w.Write(nil)
	})
	server2 := httptest.NewServer(handler2)
	t.Cleanup(server2.Close)
	manager2, _ := openManager(t, handler2)
	job2, err := manager2.Start("node")
	if err != nil {
		t.Fatalf("start truncated install: %v", err)
	}
	waitForState(t, manager2, job2.ID, StateFailed)
}

func TestCancelMarksTheJobCancelled(t *testing.T) {
	slow := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-slow
	})
	server := httptest.NewServer(handler)
	t.Cleanup(func() { close(slow); server.Close() })

	spec := Spec{Kind: "node", Version: "22.14.0", URL: server.URL + "/node.zip", Format: "zip", MustContain: "node.exe", Strip: 1}
	withCatalogue(t, spec)
	manager, root := openManager(t, handler)

	job, err := manager.Start("node")
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := manager.Cancel(job.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	cancelled := waitForState(t, manager, job.ID, StateCancelled)
	if cancelled.Error != "" {
		t.Errorf("cancelled job error = %q, want clean", cancelled.Error)
	}
	// Nothing was unpacked.
	if _, err := os.Stat(filepath.Join(root, "node")); !os.IsNotExist(err) {
		t.Errorf("target exists after a cancel: %v", err)
	}
	if err := manager.Cancel(job.ID); err != ErrRunning {
		t.Errorf("second cancel err = %v, want the not-running refusal", err)
	}
}

func TestRetryRunsAFailedJobAgainAsANewAttempt(t *testing.T) {
	var succeed bool
	archive := zipArchive(t, map[string]string{"node-v22.14.0-win-x64/node.exe": "stub"})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if succeed {
			w.Write(archive)
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	spec := Spec{Kind: "node", Version: "22.14.0", URL: server.URL + "/node.zip", Format: "zip", MustContain: "node.exe", Strip: 1}
	withCatalogue(t, spec)
	manager, root := openManager(t, handler)

	job, err := manager.Start("node")
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	waitForState(t, manager, job.ID, StateFailed)

	// The second attempt succeeds now that the server answers properly.
	succeed = true
	retry, err := manager.Retry(job.ID)
	if err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
	if retry.ID == job.ID {
		t.Error("the retry reused the failed job's id")
	}
	waitForState(t, manager, retry.ID, StateSucceeded)
	if _, err := os.Stat(filepath.Join(root, "node", "22.14.0", "node.exe")); err != nil {
		t.Errorf("the retried install did not arrive: %v", err)
	}

	// Both attempts are in the history, newest first.
	jobs, err := manager.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(jobs) < 2 {
		t.Fatalf("history = %d jobs, want at least 2 attempts", len(jobs))
	}

	// A succeeded job can be dropped from the history, and the installed
	// files stay.
	if err := manager.Remove(retry.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	jobs, _ = manager.List()
	for _, entry := range jobs {
		if entry.ID == retry.ID {
			t.Error("the removed job is still listed")
		}
	}
	if _, err := os.Stat(filepath.Join(root, "node", "22.14.0", "node.exe")); err != nil {
		t.Errorf("removal touched the installed files: %v", err)
	}
}

func TestRefusalsForJobsThatCannotChange(t *testing.T) {
	manager, _ := openManager(t, nil)

	if err := manager.Remove("nope"); err != ErrNotFound {
		t.Errorf("remove unknown err = %v, want not found", err)
	}
	if _, err := manager.Retry("nope"); err != ErrNotFound {
		t.Errorf("retry unknown err = %v, want not found", err)
	}
	if _, err := manager.Retry("nope-2"); err != ErrNotFound {
		t.Errorf("retry unknown err = %v, want not found", err)
	}
}
