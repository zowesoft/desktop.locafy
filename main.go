package main

import (
	"database/sql"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/logs"
	"locafy.dev/desktop/internal/machine"
	"locafy.dev/desktop/internal/settings"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

const (
	logMaxBytes = 10 << 20 // rotate at 10 MiB
	logBackups  = 3
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "locafy: startup failed:", err)
		os.Exit(1)
	}
}

// run performs engine startup in order — logging, persistence, machine
// registration — and then runs the application until exit.
func run() error {
	logFile, err := setupLogging()
	if err != nil {
		return err
	}
	defer logFile.Close()
	slog.Info("engine starting")

	database, err := openDatabase()
	if err != nil {
		return err
	}
	defer database.Close()

	if err := machine.Register(database); err != nil {
		return err
	}
	slog.Info("machine record registered")

	if err := activity.Log(database, activity.ScopeMachine, "", "Engine started", ""); err != nil {
		return err
	}

	app := application.New(application.Options{
		Name:        "Locafy",
		Description: "Local-first development environment control plane",
		Services: []application.Service{
			application.NewService(settings.NewService(database)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Locafy",
		Width:  1280,
		Height: 800,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	return app.Run()
}

// openDatabase opens the control-plane database at its platform location
// and brings the schema up to date.
func openDatabase() (*sql.DB, error) {
	path, err := db.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	database, err := db.Open(path)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(database); err != nil {
		database.Close()
		return nil, err
	}
	slog.Info("database ready", "path", path)
	return database, nil
}

// setupLogging installs structured logging into a rotating file under the
// Locafy data directory, mirrored to stderr for development runs.
func setupLogging() (*logs.Rotator, error) {
	home, err := db.DataHome()
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	dir := filepath.Join(home, db.AppDirName(), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	logFile, err := logs.New(filepath.Join(dir, "locafy.log"), logMaxBytes, logBackups)
	if err != nil {
		return nil, err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.MultiWriter(logFile, os.Stderr), nil)))
	return logFile, nil
}
