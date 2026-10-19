// phase: 09

//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// The way Go programs call the system on Windows: LazyDLL/LazyProc, Call with uintptr arguments,
// the error that Call returns, and the helpers of package syscall (UTF16, Handle, Errno).
func main() {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	pid := k32.NewProc("GetCurrentProcessId")
	r, _, err := pid.Call()
	fmt.Println(int(r) == os.Getpid(), err)

	// A call with a string and an output buffer (more than four arguments).
	get := k32.NewProc("GetEnvironmentVariableW")
	os.Setenv("GPWIN_DLL", "from the environment")
	name, _ := syscall.UTF16PtrFromString("GPWIN_DLL")
	buf := make([]uint16, 64)
	n, _, err := get.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	fmt.Println(n, syscall.UTF16ToString(buf[:n]), err)

	// A call that fails: Call's error is the Errno, always non-nil.
	missing, _ := syscall.UTF16PtrFromString(`C:\gocpp-no-such-dir\file`)
	attrs := k32.NewProc("GetFileAttributesW")
	a, _, err := attrs.Call(uintptr(unsafe.Pointer(missing)))
	fmt.Println(a == 0xffffffff, err, err == syscall.ERROR_PATH_NOT_FOUND)
	var errno syscall.Errno
	fmt.Println(errors.As(err, &errno), uintptr(errno))

	// Procedures and libraries that do not exist.
	fmt.Println(k32.NewProc("GocppNoSuchProc").Find())
	_, err = syscall.LoadDLL("gocpp-no-such-library.dll")
	fmt.Println(err)
	var de *syscall.DLLError
	fmt.Println(errors.As(err, &de), de.ObjName, errors.Is(err, syscall.Errno(126)))
	d, err := syscall.LoadDLL("kernel32.dll")
	fmt.Println(err, d.Name)
	p, err := d.FindProc("GetTickCount64")
	fmt.Println(err, p.Name)
	t1, _, _ := p.Call()
	t2, _, _ := p.Call()
	fmt.Println(t2 >= t1, t1 > 0)

	// SyscallN and the fixed forms, with the address of a procedure.
	r1, _, e1 := syscall.SyscallN(pid.Addr())
	fmt.Println(int(r1) == os.Getpid(), e1)
	r1, _, e1 = syscall.Syscall(pid.Addr(), 0, 0, 0, 0)
	fmt.Println(int(r1) == os.Getpid(), e1)

	// UTF-16.
	u, err := syscall.UTF16FromString("h\u00e9llo \U0001F642")
	fmt.Println(u, err)
	fmt.Println(syscall.UTF16ToString(u))
	_, err = syscall.UTF16FromString("a\x00b")
	fmt.Println(err)
	fmt.Println(syscall.UTF16ToString([]uint16{0xd800, 'x'}) == "\xed\xa0\x80x")
	fmt.Println(syscall.InvalidHandle, syscall.Handle(5))

	// The process and the environment through package syscall.
	fmt.Println(syscall.Getpid() == os.Getpid(), syscall.Getppid() == os.Getppid())
	v, ok := syscall.Getenv("gpwin_dll")
	fmt.Println(v, ok)
	wd, err := syscall.Getwd()
	w2, _ := os.Getwd()
	fmt.Println(wd == w2, err)
}

// Output:
// true The operation completed successfully.
// 20 from the environment The operation completed successfully.
// true The system cannot find the path specified. true
// true 3
// Failed to find GocppNoSuchProc procedure in kernel32.dll: The specified procedure could not be found.
// Failed to load gocpp-no-such-library.dll: The specified module could not be found.
// true gocpp-no-such-library.dll true
// <nil> kernel32.dll
// <nil> GetTickCount64
// true true
// true The operation completed successfully.
// true The operation completed successfully.
// [104 233 108 108 111 32 55357 56898 0] <nil>
// héllo 🙂
// invalid argument
// true
// 18446744073709551615 5
// true true
// from the environment true
// true <nil>
