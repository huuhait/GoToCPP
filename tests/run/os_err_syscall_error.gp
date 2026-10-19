// phase: 09

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
// open: no such file or directory
// open no such file or directory true false
// true true
// read: resource temporarily unavailable true true
// false
