// phase: 09

//go:build linux

package main

import (
	"fmt"
	"syscall"
)

func main() {
	for _, e := range []syscall.Errno{1, 2, 3, 4, 5, 9, 11, 12, 13, 17, 20, 21, 22, 24, 28, 30, 32, 36, 39, 40, 95, 110, 0, 133, 200} {
		fmt.Printf("%d %q\n", uintptr(e), e.Error())
	}
	fmt.Println(syscall.ENOENT, syscall.EEXIST, syscall.EPERM, syscall.EPIPE)
	fmt.Println(syscall.EAGAIN.Temporary(), syscall.EAGAIN.Timeout(), syscall.ENOENT.Temporary(), syscall.EINTR.Temporary(), syscall.ETIMEDOUT.Timeout())
	fmt.Println(syscall.SIGINT, syscall.SIGKILL, syscall.SIGPIPE, syscall.SIGTERM, syscall.Signal(0), syscall.Signal(77))
	fmt.Printf("%v %d %s\n", syscall.SIGHUP, syscall.SIGHUP, syscall.SIGUSR1)
}

// Output:
// 1 "operation not permitted"
// 2 "no such file or directory"
// 3 "no such process"
// 4 "interrupted system call"
// 5 "input/output error"
// 9 "bad file descriptor"
// 11 "resource temporarily unavailable"
// 12 "cannot allocate memory"
// 13 "permission denied"
// 17 "file exists"
// 20 "not a directory"
// 21 "is a directory"
// 22 "invalid argument"
// 24 "too many open files"
// 28 "no space left on device"
// 30 "read-only file system"
// 32 "broken pipe"
// 36 "file name too long"
// 39 "directory not empty"
// 40 "too many levels of symbolic links"
// 95 "operation not supported"
// 110 "connection timed out"
// 0 "errno 0"
// 133 "errno 133"
// 200 "errno 200"
// no such file or directory file exists operation not permitted broken pipe
// true true false true true
// interrupt killed broken pipe terminated signal 0 signal 77
// hangup 1 user defined signal 1
