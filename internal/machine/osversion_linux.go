//go:build linux

package machine

// Linux exposes no product-version sysctl, so the kernel release that
// osVersion reads from uname is the version this platform reports.
func osProductVersion() string { return "" }
