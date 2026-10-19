// phase: 09

//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func main() {
	fmt.Println(os.NewSyscallError("open", nil) == nil)
	err := os.NewSyscallError("open", syscall.ENOENT)
	fmt.Println(err)
	se := err.(*os.SyscallError)
	fmt.Println(se.Syscall, se.Err, se.Unwrap() == syscall.ENOENT, se.Timeout())
	fmt.Println(errors.Is(err, os.ErrNotExist), os.IsNotExist(err))
	t := os.NewSyscallError("read", syscall.EAGAIN)
	fmt.Println(t, t.(*os.SyscallError).Timeout(), os.IsTimeout(t))
	fmt.Println(os.NewSyscallError("x", errors.New("plain")).(*os.SyscallError).Timeout())
}

// Output:
// true
// open: The system cannot find the file specified.
// open The system cannot find the file specified. true false
// true true
// read: resource temporarily unavailable true true
// false
