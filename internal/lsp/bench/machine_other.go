//go:build !windows

package bench

// describePlatform adds what /proc does not give; nothing on Linux.
func describePlatform(m *Machine) {}
