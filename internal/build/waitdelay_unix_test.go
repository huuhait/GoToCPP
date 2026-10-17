//go:build unix

package build

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// holdersLeft returns the pids that the fake compiler at path (holdsOutput) wrote to
// path.holder and that still run (a zombie counts as gone: Kill(pid, 0) works on it, so its
// state is read).
func holdersLeft(path string) []int {
	data, _ := os.ReadFile(path + ".holder")
	var left []int
	for f := range strings.FieldsSeq(string(data)) {
		pid, err := strconv.Atoi(f)
		if err != nil || syscall.Kill(pid, 0) != nil {
			continue
		}
		if stat, err := os.ReadFile("/proc/" + f + "/stat"); err == nil {
			if i := strings.LastIndexByte(string(stat), ')'); i >= 0 && i+2 < len(stat) && stat[i+2] == 'Z' {
				continue
			}
		}
		left = append(left, pid)
	}
	return left
}

// killHolders kills what holdersLeft finds.
func killHolders(path string) {
	for _, pid := range holdersLeft(path) {
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

// waitUntil waits up to d for cond, and reports whether it held.
func waitUntil(d time.Duration, cond func() bool) bool {
	for stop := time.Now().Add(d); ; time.Sleep(10 * time.Millisecond) {
		if cond() {
			return true
		}
		if time.Now().After(stop) {
			return false
		}
	}
}
