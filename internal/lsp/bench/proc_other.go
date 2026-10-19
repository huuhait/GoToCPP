//go:build !linux && !windows

package bench

import "os/exec"

func setProcAttr(cmd *exec.Cmd) {}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// procMemory is not known on this system.
func procMemory(pid int) (rss, peak int64) { return 0, 0 }
