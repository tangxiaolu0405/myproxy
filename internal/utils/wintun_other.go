//go:build !windows

package utils

// EnsureWintunForTUN 非 Windows 无需 wintun.dll。
func EnsureWintunForTUN() error {
	return nil
}
