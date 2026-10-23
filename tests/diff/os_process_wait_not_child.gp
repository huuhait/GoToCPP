// phase: 09
//go:build linux

// Process.Wait first blocks in waitid(2) (Go's pidWait), so a process that is not a child fails
// with "waitid: no child processes", not "wait: ..." (dev.syscalldiffs).

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func main() {
	for _, pid := range []int{1, os.Getpid(), 1 << 22} {
		p, err := os.FindProcess(pid)
		fmt.Println(err)
		st, err := p.Wait()
		fmt.Println(st, err, errors.Is(err, syscall.ECHILD))
		var se *os.SyscallError
		fmt.Println(errors.As(err, &se) && se.Syscall == "waitid")
		fmt.Println(p.Signal(syscall.Signal(0)))
	}
	p, _ := os.FindProcess(1)
	p.Release()
	_, err := p.Wait()
	fmt.Println(err)
}
