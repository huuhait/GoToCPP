//go:build !unix && !windows

package toolchain

// processAlive says yes here: a download folder is then swept by its age only.
func processAlive(int) bool { return true }
