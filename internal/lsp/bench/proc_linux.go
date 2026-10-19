package bench

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// setProcAttr makes the server a process group of its own that dies with
// the benchmark (also when it is killed): no language server outlives the
// run.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}

// killGroup kills the process group of cmd.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// procMemory returns the resident set of the process pid now and its peak, in bytes, from /proc.
func procMemory(pid int) (rss, peak int64) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		var kb int64
		switch {
		case strings.HasPrefix(line, "VmRSS:"):
			fmt.Sscanf(strings.TrimPrefix(line, "VmRSS:"), "%d", &kb)
			rss = kb << 10
		case strings.HasPrefix(line, "VmHWM:"):
			fmt.Sscanf(strings.TrimPrefix(line, "VmHWM:"), "%d", &kb)
			peak = kb << 10
		}
	}
	return rss, peak
}
