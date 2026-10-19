// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	fmt.Println(os.Getpid() > 1, os.Getppid() > 0, os.Getpid() != os.Getppid())
	fmt.Println(os.Getpid() == syscall.Getpid(), os.Getppid() == syscall.Getppid())
	fmt.Println(os.Getuid() == syscall.Getuid(), os.Geteuid() == syscall.Geteuid(), os.Getgid() == syscall.Getgid(), os.Getegid() == syscall.Getegid())
	fmt.Println(os.Getuid() >= 0, os.Getgid() >= 0)
	gs, err := os.Getgroups()
	fmt.Println(len(gs) >= 0, err)
	p := os.Getpagesize()
	fmt.Println(p >= 4096, p&(p-1) == 0)
}

// Output:
// true true true
// true true
// true true true true
// false false
// true getgroups: not supported by windows
// true true
