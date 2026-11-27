// Package diagnostics assembles the bundle an operator reviews and exports when
// reporting a problem: version and machine information, the stored settings, the
// recent activity and captured log lines, and the row counts of the control
// plane. Every file is masked on the way out, so a password that reached the
// settings table or a log line never leaves the machine.
package diagnostics

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/logs"
)

// FileViewLimit is how many characters of a file the preview returns. The
// bundle itself is complete; the interface shows a window into each file so the
// review stays readable.
const FileViewLimit = 4000

// File is one member of the bundle as the interface reviews it.
type File struct {
	// Name is the member name inside the archive.
	Name string `json:"name"`
	// Content is the masked text the reviewer sees.
	Content string `json:"content"`
	// Bytes is the full size of the member, which can exceed len(Content).
	Bytes int `json:"bytes"`
	// Truncated reports that Content is only the beginning of the member.
	Truncated bool `json:"truncated"`
	// Masked reports that at least one value was replaced with a redaction.
	Masked bool `json:"masked"`
}

// Bundle is a prepared diagnostics archive, held in memory until the operator
// confirms the export.
type Bundle struct {
	GeneratedAt string `json:"generatedAt"`
	Files       []File `json:"files"`
}

// Input carries what the bundle needs from the surroundings.
type Input struct {
	Database  *sql.DB
	LogLines  []logs.Entry
	DataDir   string
	LogPath   string
	Version   string
	StartedAt time.Time
	Now       time.Time
}

// Build assembles the bundle in memory. Masking runs on every file, including
// the log lines that were masked when they were captured, because a setting or
// an environment value can carry a secret that the line-level rules missed.
func Build(in Input) (Bundle, error) {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	bundle := Bundle{GeneratedAt: now.UTC().Format(time.RFC3339)}

	add := func(name string, lines ...string) {
		bundle.Files = append(bundle.Files, maskedFile(name, strings.Join(lines, "\n")+"\n"))
	}

	hostname, _ := os.Hostname()
	add("system.txt",
		"Locafy diagnostics",
		"generated: "+now.UTC().Format(time.RFC3339),
		"version: "+versionOrUnknown(in.Version),
		"hostname: "+hostname,
		"platform: "+runtime.GOOS+"/"+runtime.GOARCH,
		"go: "+runtime.Version(),
		"data directory: "+in.DataDir,
		"database: "+filepath.Join(in.DataDir, "locafy.db"),
		"engine log: "+in.LogPath,
		"started: "+formatTime(in.StartedAt),
		"goroutines: "+fmt.Sprint(runtime.NumGoroutine()),
	)

	counts, err := tableCounts(in.Database)
	if err != nil {
		return Bundle{}, err
	}
	add("counts.txt", counts...)

	add("settings.txt", settingsLines(in.Database)...)

	activity, err := activityLines(in.Database, 200)
	if err != nil {
		return Bundle{}, err
	}
	add("activity.txt", activity...)

	logLines := make([]string, 0, len(in.LogLines))
	for _, entry := range in.LogLines {
		logLines = append(logLines, strings.Join([]string{entry.Time, entry.Level, entry.Layer + "/" + entry.Source, entry.Line}, "\t"))
	}
	if len(logLines) == 0 {
		logLines = append(logLines, "(no output captured)")
	}
	add("logs.txt", logLines...)

	return bundle, nil
}

// WriteArchive writes the bundle to path as a zip archive. The files are stored
// uncompressed: a diagnostics archive is small and a reviewer can read it with
// any unzip tool.
func WriteArchive(bundle Bundle, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("prepare diagnostics folder: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create diagnostics archive: %w", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for _, entry := range bundle.Files {
		member, err := writer.Create(entry.Name)
		if err != nil {
			return fmt.Errorf("add %s to diagnostics archive: %w", entry.Name, err)
		}
		if _, err := member.Write([]byte(entry.Content)); err != nil {
			return fmt.Errorf("write %s: %w", entry.Name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish diagnostics archive: %w", err)
	}
	return file.Close()
}

// DefaultFilename names the archive after the moment it was made, so two
// reports from one machine never overwrite each other.
func DefaultFilename(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return "locafy-diagnostics-" + now.Local().Format("20060102-150405") + ".zip"
}

func maskedFile(name, content string) File {
	masked, changed := maskText(content)
	file := File{Name: name, Content: masked, Bytes: len(masked), Masked: changed}
	if len(masked) > FileViewLimit {
		file.Content = masked[:FileViewLimit]
		file.Truncated = true
	}
	return file
}

// maskText applies the log masking rules line by line and reports whether any
// line changed.
func maskText(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	changed := false
	for index, line := range lines {
		if masked, didMask := logs.Mask(line); didMask {
			lines[index] = masked
			changed = true
		}
	}
	return strings.Join(lines, "\n"), changed
}

func tableCounts(database *sql.DB) ([]string, error) {
	lines := []string{"Control-plane tables"}
	if database == nil {
		return append(lines, "(no database)"), nil
	}
	names := map[string]string{
		"projects":                  `SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL`,
		"applications":              `SELECT COUNT(*) FROM applications WHERE deleted_at IS NULL`,
		"services":                  `SELECT COUNT(*) FROM services WHERE deleted_at IS NULL`,
		"activity entries":          `SELECT COUNT(*) FROM activity_logs`,
		"schema migrations applied": `SELECT COUNT(*) FROM schema_migrations`,
	}
	order := []string{"projects", "applications", "services", "activity entries", "schema migrations applied"}
	for _, label := range order {
		var count int
		query, known := names[label]
		if !known {
			continue
		}
		if err := database.QueryRow(query).Scan(&count); err != nil {
			// A missing table is information too: report it instead of failing
			// the whole bundle.
			lines = append(lines, fmt.Sprintf("%s: unavailable (%v)", label, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %d", label, count))
	}
	return lines, nil
}

func settingsLines(database *sql.DB) []string {
	if database == nil {
		return []string{"Stored settings", "(no database)"}
	}
	rows, err := database.Query(`SELECT key, value, value_type FROM settings ORDER BY key`)
	if err != nil {
		return []string{"Stored settings", fmt.Sprintf("(could not read settings: %v)", err)}
	}
	defer rows.Close()

	lines := []string{"Stored settings"}
	for rows.Next() {
		var key, value, kind string
		if err := rows.Scan(&key, &value, &kind); err != nil {
			continue
		}
		lines = append(lines, key+" ["+kind+"] = "+value)
	}
	if len(lines) == 1 {
		lines = append(lines, "(none stored)")
	}
	return lines
}

func activityLines(database *sql.DB, limit int) ([]string, error) {
	lines := []string{"Recent activity, newest first"}
	if database == nil {
		return append(lines, "(no database)"), nil
	}
	rows, err := database.Query(`SELECT scope_type, coalesce(detail, ''), action, created_at
		FROM activity_logs ORDER BY created_at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read activity for diagnostics: %w", err)
	}
	defer rows.Close()

	added := 0
	for rows.Next() {
		var scope, detail, action, at string
		if err := rows.Scan(&scope, &detail, &action, &at); err != nil {
			continue
		}
		if detail != "" {
			action += " — " + detail
		}
		lines = append(lines, at+"\t"+scope+"\t"+action)
		added++
	}
	if added == 0 {
		lines = append(lines, "(no activity recorded)")
	}
	return lines, rows.Err()
}

func versionOrUnknown(version string) string {
	if strings.TrimSpace(version) == "" {
		return "development"
	}
	return version
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.UTC().Format(time.RFC3339)
}

// DataDir reports the folder the engine owns, for the system summary.
func DataDir() string {
	home, err := db.DataHome()
	if err != nil {
		return ""
	}
	return filepath.Join(home, db.AppDirName())
}
