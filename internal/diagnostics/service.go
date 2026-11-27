package diagnostics

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"locafy.dev/desktop/internal/logs"
)

// ErrCancelled is returned when the operator closes the save dialog without
// choosing a file. It is an ordinary outcome, not a failure.
var ErrCancelled = errors.New("diagnostics export cancelled")

// Service prepares and exports diagnostics bundles. Preparing is separate from
// exporting on purpose: the interface shows the prepared bundle first, and only
// an explicit confirmation reaches the save dialog.
type Service struct {
	database  *sql.DB
	store     *logs.Store
	startedAt time.Time
	logPath   string
	version   string
}

// NewService returns a diagnostics service. store may be nil in a build without
// log capture; the bundle then reports no captured output.
func NewService(database *sql.DB, store *logs.Store, startedAt time.Time, logPath string) *Service {
	return &Service{database: database, store: store, startedAt: startedAt, logPath: logPath}
}

// Preview prepares the bundle and returns it for review. Nothing is written.
func (s *Service) Preview() (Bundle, error) {
	return Build(Input{
		Database:  s.database,
		LogLines:  s.logLines(),
		DataDir:   DataDir(),
		LogPath:   s.logPath,
		Version:   s.version,
		StartedAt: s.startedAt,
		Now:       time.Now(),
	})
}

// Export asks for a destination and writes the reviewed bundle there. It
// rebuilds the bundle so the archive is exactly what the current state reports;
// the review the operator approved is of the same files.
func (s *Service) Export() (string, error) {
	bundle, err := s.Preview()
	if err != nil {
		return "", err
	}
	path, err := choosePath(DefaultFilename(time.Now()))
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", ErrCancelled
	}
	if err := WriteArchive(bundle, path); err != nil {
		return "", err
	}
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("diagnostics archive was not written: %w", err)
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("diagnostics archive is empty: %s", path)
	}
	return path, nil
}

func (s *Service) logLines() []logs.Entry {
	if s.store == nil {
		return nil
	}
	return s.store.Query(logs.Filter{Limit: logs.DefaultCapacity})
}

// choosePath opens the native save dialog. An empty path means it was closed
// without a choice.
func choosePath(defaultName string) (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("no application window is available for the save dialog")
	}
	dialog := app.Dialog.SaveFile()
	dialog.SetOptions(&application.SaveFileDialogOptions{
		Title:    "Save diagnostics bundle",
		Filename: defaultName,
		Filters:  []application.FileFilter{{DisplayName: "Zip archive", Pattern: "*.zip"}},
	})
	if window := app.Window.Current(); window != nil {
		dialog.AttachToWindow(window)
	}
	return dialog.PromptForSingleSelection()
}
