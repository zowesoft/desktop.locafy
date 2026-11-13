//go:build linux || darwin

package machine

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func osVersion() (string, error) {
	if product, err := unix.Sysctl("kern.osproductversion"); err == nil && product != "" {
		return product, nil
	}
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return "", err
	}
	release := make([]byte, 0, len(uname.Release))
	for _, b := range uname.Release {
		if b == 0 {
			break
		}
		release = append(release, byte(b))
	}
	if len(release) == 0 {
		return "", fmt.Errorf("empty kernel release")
	}
	return string(release), nil
}
