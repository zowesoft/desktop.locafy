//go:build darwin

package machine

import "golang.org/x/sys/unix"

func osProductVersion() string {
	product, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return ""
	}
	return product
}
