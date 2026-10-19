//go:build linux

package main

import (
	"os"
	"strconv"
	"syscall"
)

func setPdeathsig(a *syscall.SysProcAttr) { a.Pdeathsig = syscall.SIGKILL }

// readVmHWM returns the VmHWM line of /proc/<pid>/status in KiB, 0 when the
// process is gone or has no such line.
func readVmHWM(pid int) int64 {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return 0
	}
	return parseVmHWM(string(data))
}
