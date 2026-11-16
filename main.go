package main

import (
	"database/sql"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"locafy.dev/desktop/internal/activity"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/dialogs"
	"locafy.dev/desktop/internal/logs"
	"locafy.dev/desktop/internal/machine"
	"locafy.dev/desktop/internal/projects"
	"locafy.dev/desktop/internal/settings"
	"locafy.dev/desktop/internal/status"
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
	// Width and height match the window in the reference screenshot, whose
	// frame is about 800 by 611 device-independent pixels at 150% scaling.
	windowDefaultWidth  = 800
	windowDefaultHeight = 611
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
	startedAt := time.Now()

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
			application.NewService(status.NewService(database, startedAt)),
			application.NewService(projects.NewService(database)),
			application.NewService(dialogs.NewService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			AdditionalBrowserArgs: debugBrowserArgs(),
		},
	})

	startPosition, startX, startY := windowStartPosition()

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Locafy",
		Width:           windowDefaultWidth,
		Height:          windowDefaultHeight,
		MinWidth:        windowMinWidth,
		MinHeight:       windowMinHeight,
		InitialPosition: startPosition,
		X:               startX,
		Y:               startY,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		// Matches the dark palette's background, which is the default theme, so
		// the webview never paints a different colour than the page.
		BackgroundColour: application.NewRGB(0x11, 0x11, 0x16),
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

// debugBrowserArgs opens a local debugging port on the webview when
// LOCAFY_DEBUG_PORT is set, which lets a developer inspect or capture the live
// window. It stays closed otherwise: the port would let any local process drive
// the interface. Wails clears the equivalent environment variable, so the port
// has to be passed as a browser argument.
func debugBrowserArgs() []string {
	port := strings.TrimSpace(os.Getenv("LOCAFY_DEBUG_PORT"))
	if port == "" {
		return nil
	}

	return []string{"--remote-debugging-port=" + port}
}

// fitToWorkArea shrinks the window to the primary screen's work area. Window
// sizes and the reported work area are both in device-independent pixels, so
// no conversion is needed.
func fitToWorkArea(window *application.WebviewWindow, app *application.App) {
	screen := app.Screen.GetPrimary()
	if screen == nil {
		return
	}
	const margin = 40 // leaves the window frame clear of the work area edges
	width := min(windowDefaultWidth, screen.WorkArea.Width-margin)
	height := min(windowDefaultHeight, screen.WorkArea.Height-margin)
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
