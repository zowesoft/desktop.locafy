package scaffold

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// JobSummary is one recorded scaffold run as the history view lists it. The
// recipe label is resolved here so the view does not have to join the
// catalogue itself.
type JobSummary struct {
	ID           string
	ProjectID    string
	RecipeID     string
	RecipeLabel  string
	Directory    string
	TargetPath   string
	Status       string
	Error        string
	LogPath      string
	LogAvailable bool
	CreatedAt    string
	UpdatedAt    string
}

// maxLogBytes bounds how much saved output is returned to the view; an
// installer can write megabytes, and the tail is what diagnoses a failure.
const maxLogBytes = 512 * 1024

// History returns recent scaffold runs, newest first.
func (s *Service) History(limit int) ([]JobSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.database.Query(`SELECT id, COALESCE(project_id, ''), recipe_id, directory, target_path,
			status, error, log_path, created_at, updated_at
		FROM scaffold_jobs ORDER BY created_at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list scaffold jobs: %w", err)
	}
	defer rows.Close()

	var jobs []JobSummary
	for rows.Next() {
		var job JobSummary
		if err := rows.Scan(&job.ID, &job.ProjectID, &job.RecipeID, &job.Directory, &job.TargetPath,
			&job.Status, &job.Error, &job.LogPath, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan scaffold job: %w", err)
		}
		job.RecipeLabel = s.recipeLabel(job.RecipeID)
		job.LogAvailable = fileExists(job.LogPath)
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// JobLog returns a run's saved output. A run that has not written its file yet
// falls back to the output held in memory.
func (s *Service) JobLog(jobID string) (string, error) {
	path, err := s.storedLogPath(jobID)
	if err != nil {
		return "", err
	}
	if path != "" {
		payload, readErr := os.ReadFile(path)
		if readErr == nil {
			return tail(payload), nil
		}
		if !errors.Is(readErr, fs.ErrNotExist) {
			return "", fmt.Errorf("read scaffold log: %w", readErr)
		}
	}

	_, lines, err := s.manager.Get(jobID)
	if err != nil {
		return "", fmt.Errorf("scaffold log for job %s is not available", jobID)
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("scaffold log for job %s is empty", jobID)
	}
	return strings.Join(lines, "\n"), nil
}

// OpenLog hands the saved log file to the OS default application.
func (s *Service) OpenLog(jobID string) error {
	path, err := s.storedLogPath(jobID)
	if err != nil {
		return err
	}
	if !fileExists(path) {
		return fmt.Errorf("the log file for job %s is not on disk", jobID)
	}
	if err := openLogFile(path); err != nil {
		return fmt.Errorf("open scaffold log %q: %w", path, err)
	}
	return nil
}

// storedLogPath reads a job's log path from the database, which is the record
// that survives closing the application.
func (s *Service) storedLogPath(jobID string) (string, error) {
	var path string
	err := s.database.QueryRow(`SELECT log_path FROM scaffold_jobs WHERE id = ?`, jobID).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("scaffold job %s not found", jobID)
	}
	if err != nil {
		return "", fmt.Errorf("read scaffold job %s: %w", jobID, err)
	}
	return path, nil
}

func (s *Service) recipeLabel(recipeID string) string {
	if recipe, err := s.catalogue.Find(recipeID); err == nil {
		return recipe.Label
	}
	return recipeID
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// tail keeps the end of a long log, where the failure is, and says so.
func tail(payload []byte) string {
	if len(payload) <= maxLogBytes {
		return string(payload)
	}
	cut := payload[len(payload)-maxLogBytes:]
	if index := strings.IndexByte(string(cut), '\n'); index >= 0 {
		cut = cut[index+1:]
	}
	return "… earlier output omitted …\n" + string(cut)
}

// openLogFile is a seam for tests: the real implementation hands the path to
// the OS, which is the only way to open a file from inside a webview.
var openLogFile = func(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
