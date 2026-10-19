//go:build windows

package toolchain

import "syscall"

// processAlive reports whether a process with this number is running.
func processAlive(pid int) bool {
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return err == syscall.ERROR_ACCESS_DENIED // it exists, but is not ours to look at
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return true
	}
	return code == 259 // STILL_ACTIVE
}
