//go:build windows

package machine

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func osVersion() (string, error) {
	info := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", info.MajorVersion, info.MinorVersion, info.BuildNumber), nil
}
