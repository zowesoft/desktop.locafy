//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	procGetDpiForSystem       = user32.NewProc("GetDpiForSystem")
)

const spiGetWorkArea = 0x0030

// windowStartPosition centres the window on the primary screen's work area.
// Wails positions a minimised window from its parked rectangle, so the
// position is computed here instead of delegated to the default centering.
func windowStartPosition() (application.WindowStartPosition, int, int) {
	var workArea winRect
	ok, _, _ := procSystemParametersInfoW.Call(
		spiGetWorkArea, 0, uintptr(unsafe.Pointer(&workArea)), 0,
	)
	dpi, _, _ := procGetDpiForSystem.Call()
	if ok == 0 || dpi == 0 {
		return application.WindowCentered, 0, 0
	}

	scale := float64(dpi) / 96
	dip := func(physical int32) int { return int(float64(physical) / scale) }
	x := dip(workArea.Left) + (dip(workArea.Right-workArea.Left)-windowDefaultWidth)/2
	y := dip(workArea.Top) + (dip(workArea.Bottom-workArea.Top)-windowDefaultHeight)/2
	return application.WindowXY, x, y
}
