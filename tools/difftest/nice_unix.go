//go:build unix

package main

import "syscall"

// setNice lowers the priority of the tool and of everything it starts.
func setNice(n int) {
	if n > 0 {
		syscall.Setpriority(syscall.PRIO_PROCESS, 0, n)
	}
}
