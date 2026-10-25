//go:build linux

package main

import (
	"os"
	"strconv"
	"strings"
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

// parseVmHWM returns the VmHWM line of a /proc/<pid>/status text in KiB, 0
// without one.
func parseVmHWM(status string) int64 {
	for _, line := range strings.Split(status, "\n") {
		if rest, ok := strings.CutPrefix(line, "VmHWM:"); ok {
			f := strings.Fields(rest)
			if len(f) >= 1 {
				v, _ := strconv.ParseInt(f[0], 10, 64)
				return v
			}
		}
	}
	return 0
}
