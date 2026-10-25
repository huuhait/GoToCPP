//go:build !unix

package main

import (
	"os"
	"os/exec"
	"time"
)

func setupChild(cmd *exec.Cmd) { cmd.WaitDelay = 2 * time.Second }

func usage(ps *os.ProcessState) (time.Duration, int64) {
	return ps.UserTime() + ps.SystemTime(), 0
}

func readVmHWM(int) int64 { return 0 }
