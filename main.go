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
	"github.com/wailsapp/wails/v3/pkg/events"

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

	// Window sizes are device-independent pixels. The window opens at the
	// default size and is fitted to the screen's work area once it is shown.
	windowDefaultWidth  = 1100
	windowDefaultHeight = 760
	windowMinWidth      = 720
	windowMinHeight     = 400
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

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Locafy",
		Width:           windowDefaultWidth,
		Height:          windowDefaultHeight,
		MinWidth:        windowMinWidth,
		MinHeight:       windowMinHeight,
		InitialPosition: application.WindowCentered,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
		// The caption bar is drawn by Windows, so it ignores the app theme
		// unless it is told otherwise. These are the palette's background,
		// border and text colours.
		Windows: application.WindowsWindow{
			Theme: application.Dark,
			CustomTheme: application.ThemeSettings{
				DarkModeActive: &application.WindowTheme{
					TitleBarColour:  application.NewRGBPtr(0x11, 0x11, 0x16),
					BorderColour:    application.NewRGBPtr(0x34, 0x34, 0x40),
					TitleTextColour: application.NewRGBPtr(0xF5, 0xF3, 0xF8),
				},
				DarkModeInactive: &application.WindowTheme{
					TitleBarColour:  application.NewRGBPtr(0x11, 0x11, 0x16),
					BorderColour:    application.NewRGBPtr(0x34, 0x34, 0x40),
					TitleTextColour: application.NewRGBPtr(0xA5, 0xA1, 0xB2),
				},
			},
		},
	})

	// A fixed size overflows a scaled display: the values are device
	// independent while screens are often smaller than their scale suggests.
	// Fit to the work area instead, and let the user maximise from there.
	window.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		fitToWorkArea(window, app)
	})

	return app.Run()
}

// fitToWorkArea shrinks the window to the primary screen's work area. Window
// sizes are in device-independent pixels while the reported work area is in
// physical pixels, so the screen's scale factor converts between the two.
func fitToWorkArea(window *application.WebviewWindow, app *application.App) {
	screen := app.Screen.GetPrimary()
	if screen == nil {
		return
	}
	scale := float64(screen.ScaleFactor)
	if scale <= 0 {
		scale = 1
	}
	const margin = 40 // leaves the window frame clear of the work area edges
	width := min(windowDefaultWidth, int(float64(screen.WorkArea.Width)/scale)-margin)
	height := min(windowDefaultHeight, int(float64(screen.WorkArea.Height)/scale)-margin)
	window.SetSize(max(width, windowMinWidth), max(height, windowMinHeight))
	window.Center()
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
