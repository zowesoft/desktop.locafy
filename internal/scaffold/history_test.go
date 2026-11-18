package scaffold

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"locafy.dev/desktop/internal/db"
)

func historyService(t *testing.T) (*Service, *sql.DB, string) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "locafy.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	logDir := t.TempDir()
	service := NewService(database)
	service.manager.logDir = logDir
	return service, database, logDir
}

func seedJob(t *testing.T, database *sql.DB, id string, recipeID string, status string, logPath string) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO scaffold_jobs (id, recipe_id, directory, target_path, status, error, log_path)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, recipeID, ".", `C:\locafy\projects\`+id, status,
		map[string]string{JobFailed: "the installer failed"}[status], logPath); err != nil {
		t.Fatalf("seed job: %v", err)
	}
}

func TestHistoryListsRunsNewestFirstWithRecipeLabels(t *testing.T) {
	service, database, logDir := historyService(t)

	first := filepath.Join(logDir, "scaffold-first.log")
	if err := os.WriteFile(first, []byte("first output\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	seedJob(t, database, "first", "laravel", JobSucceeded, first)
	seedJob(t, database, "second", "django", JobFailed, filepath.Join(logDir, "missing.log"))

	jobs, err := service.History(10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("history holds %d jobs, want 2", len(jobs))
	}
	// Both rows are created in the same second, so the later insert leads.
	if jobs[0].ID != "second" || jobs[1].ID != "first" {
		t.Fatalf("order = [%s %s], want the newest run first", jobs[0].ID, jobs[1].ID)
	}
	if jobs[1].RecipeLabel != "Laravel" {
		t.Fatalf("recipe label = %q, want Laravel", jobs[1].RecipeLabel)
	}
	if jobs[1].LogAvailable != true {
		t.Fatal("first job's log reported missing, want present")
	}
	if jobs[0].LogAvailable {
		t.Fatal("second job's log reported present, want missing: it was never written")
	}
	if jobs[0].Error != "the installer failed" {
		t.Fatalf("error = %q, want the recorded failure", jobs[0].Error)
	}
}

func TestHistoryDefaultsAndCapsItsLimit(t *testing.T) {
	service, database, logDir := historyService(t)

	for index := 0; index < 3; index++ {
		seedJob(t, database, "job-"+string(rune('a'+index)), "vite", JobSucceeded, filepath.Join(logDir, "none.log"))
	}

	jobs, err := service.History(0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("history holds %d jobs, want 3", len(jobs))
	}
	if jobs[0].RecipeLabel != "Vite" {
		t.Fatalf("recipe label = %q, want Vite", jobs[0].RecipeLabel)
	}
}

func TestJobLogReadsTheSavedFileForFinishedRuns(t *testing.T) {
	service, database, logDir := historyService(t)

	path := filepath.Join(logDir, "scaffold-finished.log")
	if err := os.WriteFile(path, []byte("$ npm install\nnpm ERR! missing script\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	seedJob(t, database, "finished", "nextjs", JobFailed, path)

	output, err := service.JobLog("finished")
	if err != nil {
		t.Fatalf("JobLog: %v", err)
	}
	if !strings.Contains(output, "npm ERR! missing script") {
		t.Fatalf("log = %q, want the saved failure", output)
	}
}

func TestJobLogFallsBackToLiveOutputAndReportsMissingLogs(t *testing.T) {
	service, database, _ := historyService(t)

	service.manager.jobs["live"] = &liveJob{job: Job{ID: "live", Status: JobRunning}, lines: []string{"$ composer install"}}
	seedJob(t, database, "live", "laravel", JobRunning, "")

	output, err := service.JobLog("live")
	if err != nil {
		t.Fatalf("JobLog live: %v", err)
	}
	if output != "$ composer install" {
		t.Fatalf("live log = %q, want the in-memory line", output)
	}

	seedJob(t, database, "gone", "vite", JobFailed, "")
	if _, err := service.JobLog("gone"); err == nil {
		t.Fatal("JobLog for a run with no output and no live lines returned no error")
	}
	if _, err := service.JobLog("unknown"); err == nil {
		t.Fatal("JobLog for an unknown job returned no error")
	}
}

func TestLongLogsKeepTheirEnd(t *testing.T) {
	payload := strings.Repeat("x", maxLogBytes) + "\nthe last word\n"
	output := tail([]byte(payload))
	if !strings.HasPrefix(output, "… earlier output omitted …") {
		t.Fatalf("output starts with %q, want the omission notice", output[:40])
	}
	if !strings.HasSuffix(output, "the last word\n") {
		t.Fatal("output dropped the end of the log")
	}
}

func TestOpenLogUsesTheStoredPathAndReportsMissingFiles(t *testing.T) {
	service, database, logDir := historyService(t)

	path := filepath.Join(logDir, "scaffold-open.log")
	if err := os.WriteFile(path, []byte("output\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	seedJob(t, database, "open", "vite", JobSucceeded, path)

	opened := ""
	restore := openLogFile
	openLogFile = func(candidate string) error {
		opened = candidate
		return nil
	}
	t.Cleanup(func() { openLogFile = restore })

	if err := service.OpenLog("open"); err != nil {
		t.Fatalf("OpenLog: %v", err)
	}
	if opened != path {
		t.Fatalf("opened %q, want %q", opened, path)
	}

	seedJob(t, database, "absent", "vite", JobFailed, filepath.Join(logDir, "absent.log"))
	if err := service.OpenLog("absent"); err == nil {
		t.Fatal("OpenLog for a missing file returned no error")
	}

	openLogFile = func(string) error { return errors.New("no handler") }
	if err := service.OpenLog("open"); err == nil {
		t.Fatal("OpenLog swallowed a failure from the OS")
	}
}
