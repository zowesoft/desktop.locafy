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
	"locafy.dev/desktop/internal/analytics"
	"locafy.dev/desktop/internal/applications"
	"locafy.dev/desktop/internal/db"
	"locafy.dev/desktop/internal/dialogs"
	"locafy.dev/desktop/internal/downloads"
	"locafy.dev/desktop/internal/logs"
	"locafy.dev/desktop/internal/machine"
	"locafy.dev/desktop/internal/network"
	"locafy.dev/desktop/internal/projects"
	"locafy.dev/desktop/internal/runtimes"
	"locafy.dev/desktop/internal/scaffold"
	"locafy.dev/desktop/internal/services"
	"locafy.dev/desktop/internal/settings"
	"locafy.dev/desktop/internal/status"
	"locafy.dev/desktop/internal/tools"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

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

	var window *application.WebviewWindow

	statusService := status.NewService(database, startedAt)
	servicesService := services.NewService(database)
	appsService := applications.NewService(database, services.SupervisorOf(servicesService))
	logStore := logs.NewStore(database, logs.DefaultCapacity)
	applications.SupervisorOf(appsService).SetLogSink(logStore)
	services.SupervisorOf(servicesService).SetLogSink(logStore)
	logsService := logs.NewService(logStore)
	analyticsService := analytics.NewService(database)
	if pruned, err := analytics.ApplyRetention(database); err != nil {
		slog.Warn("history retention could not run", "error", err)
	} else if pruned > 0 {
		slog.Info("history pruned to its retention window", "rows", pruned)
	}
	networkService := network.NewService(database)
	scaffoldService := scaffold.NewService(database)
	runtimesService := runtimes.NewService()
	downloadsService := downloads.NewService(downloads.NewManager(
		database,
		tools.ManagedRoot(),
		func(name string, data any) {
			if app := application.Get(); app != nil {
				app.Event.Emit(name, data)
			}
		},
	))

	app := application.New(application.Options{
		Name:        "Locafy",
		Description: "Local-first development environment control plane",
		Services: []application.Service{
			application.NewService(settings.NewService(database)),
			application.NewService(statusService),
			application.NewService(projects.NewService(database)),
			application.NewService(appsService),
			application.NewService(servicesService),
			application.NewService(logsService),
			application.NewService(analyticsService),
			application.NewService(networkService),
			application.NewService(scaffoldService),
			application.NewService(runtimesService),
			application.NewService(downloadsService),
			application.NewService(dialogs.NewService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		Windows: application.WindowsOptions{
			AdditionalBrowserArgs: debugBrowserArgs(),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "dev.locafy.desktop",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				window.Show()
				window.UnMinimise()
				window.Focus()
			},
		},
	})

	// Domains are served as soon as Locafy is up. A refused port is reported in
	// the Domains tab instead of blocking the window from opening.
	if _, err := networkService.StartProxy(); err != nil {
		slog.Warn("local proxy did not start", "error", err)
	}

	// A server started from Locafy must not outlive it.
	app.OnShutdown(appsService.StopAll)
	app.OnShutdown(servicesService.StopAll)
	app.OnShutdown(func() { networkService.StopProxy() })

	startPosition, startX, startY := windowStartPosition()

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Locafy",
		Width:     windowDefaultWidth,
		Height:    windowDefaultHeight,
		MinWidth:  windowMinWidth,
		MinHeight: windowMinHeight,
		// The splash in index.html covers first paint, so the window can
		// open immediately and paint brand artwork while the bundle loads.
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

	// The tray keeps Locafy reachable while it works in the background; the
	// window opens on demand and hides again when dismissed.
	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetTooltip("Locafy")
	tray.OnClick(func() {
		if window.IsVisible() {
			window.Hide()
			return
		}
		window.Show()
		window.Focus()
	})

	// The tray menu is Locafy's global surface: the window is usually hidden, so
	// the screens an operator reaches for and the actions that do not need a
	// window are one right-click away.
	menu := application.NewMenu()
	show := menu.Add("Show Locafy")
	show.OnClick(func(*application.Context) {
		window.Show()
		window.Focus()
	})

	menu.AddSeparator()
	header(menu, "Quick access")
	for _, item := range []struct {
		label string
		path  string
	}{
		{"Overview", "/"},
		{"Projects", "/projects"},
		{"Settings", "/settings"},
	} {
		goTo := menu.Add(item.label)
		goTo.OnClick(func(*application.Context) {
			openScreen(window, item.path)
		})
	}

	menu.AddSeparator()
	dataFolder := menu.Add("Open data folder")
	dataFolder.OnClick(func(*application.Context) {
		if err := statusService.OpenDataFolder(); err != nil {
			slog.Error("open data folder", "error", err)
		}
	})

	menu.AddSeparator()
	quit := menu.Add("Quit Locafy")
	quit.OnClick(func(*application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	// Minimize and close hide the window instead of closing or iconic-parking
	// it, so Locafy keeps running until it is quit from the tray menu.
	window.OnWindowEvent(events.Common.WindowMinimise, func(*application.WindowEvent) {
		window.Hide()
	})
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		window.Hide()
	})

	// The splash in index.html paints brand artwork while the bundle
	// loads, so there is no blank frame to hide here.

	// A fixed size overflows a scaled display: the values are device
	// independent while screens are often smaller than their scale suggests.
	// Fit to the work area instead, and let the user maximise from there.
	window.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		fitToWorkArea(window, app)
	})

	return app.Run()
}

// header adds a disabled entry that names the group of items below it. Windows
// tray menus have no section headers, and a disabled item is the nearest native
// equivalent that does not look like something to click.
func header(menu *application.Menu, label string) {
	item := menu.Add(label)
	item.SetEnabled(false)
}

// openScreen shows the window and moves it to a screen by hash route, so a tray
// entry lands on the screen it names instead of wherever the window was left.
func openScreen(window *application.WebviewWindow, path string) {
	window.Show()
	window.Focus()
	window.ExecJS(fmt.Sprintf("window.location.hash = %q", "#"+path))
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
