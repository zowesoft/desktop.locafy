//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// windowStartPosition asks Wails to centre the window on the primary screen.
func windowStartPosition() (application.WindowStartPosition, int, int) {
	return application.WindowCentered, 0, 0
}
