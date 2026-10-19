// phase: 09

//go:build windows

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func main() {
	// Windows error numbers have the system's own texts (FormatMessage, in English), Go's invented
	// ones (ENOENT and friends) have Go's.
	for _, code := range []syscall.Errno{2, 3, 5, 6, 32, 80, 87, 123, 145, 183, 267, 1314, 99999} {
		fmt.Println(int(code), code.Error())
	}
	fmt.Println(syscall.ENOENT.Error(), "|", syscall.EEXIST.Error(), "|", syscall.EINVAL.Error(), "|", syscall.ENOTDIR.Error())
	fmt.Println(syscall.EWINDOWS.Error())

	// errors.Is maps the system's numbers to the portable errors.
	for _, c := range []struct {
		name string
		err  error
	}{
		{"FILE_NOT_FOUND", syscall.ERROR_FILE_NOT_FOUND},
		{"PATH_NOT_FOUND", syscall.ERROR_PATH_NOT_FOUND},
		{"ACCESS_DENIED", syscall.ERROR_ACCESS_DENIED},
		{"ALREADY_EXISTS", syscall.ERROR_ALREADY_EXISTS},
		{"FILE_EXISTS", syscall.ERROR_FILE_EXISTS},
		{"DIR_NOT_EMPTY", syscall.ERROR_DIR_NOT_EMPTY},
		{"ENOENT", syscall.ENOENT},
		{"EACCES", syscall.EACCES},
		{"EEXIST", syscall.EEXIST},
		{"SHARING_VIOLATION", syscall.Errno(32)},
		{"CALL_NOT_IMPLEMENTED", syscall.Errno(120)},
	} {
		fmt.Println(c.name, errors.Is(c.err, fs.ErrNotExist), errors.Is(c.err, fs.ErrExist), errors.Is(c.err, fs.ErrPermission), errors.Is(c.err, errors.ErrUnsupported))
	}

	// The errors of package os carry them.
	_, err := os.Stat(`C:\gocpp-no-such-dir\file`)
	fmt.Println(err, errors.Is(err, fs.ErrNotExist), os.IsNotExist(err))
	var pe *fs.PathError
	fmt.Println(errors.As(err, &pe), pe.Op, pe.Err == syscall.ERROR_PATH_NOT_FOUND)
	_, err = os.Stat(`C:\Windows\gocpp-no-such-file`)
	fmt.Println(err, errors.Is(err, syscall.ERROR_FILE_NOT_FOUND))
	err = os.Mkdir(`C:\Windows`, 0o777)
	fmt.Println(err, os.IsExist(err), errors.Is(err, fs.ErrExist))
	fmt.Println(os.Remove(`C:\Windows\gocpp-no-such-file`))
}

// Output:
// 2 The system cannot find the file specified.
// 3 The system cannot find the path specified.
// 5 Access is denied.
// 6 The handle is invalid.
// 32 The process cannot access the file because it is being used by another process.
// 80 The file exists.
// 87 The parameter is incorrect.
// 123 The filename, directory name, or volume label syntax is incorrect.
// 145 The directory is not empty.
// 183 Cannot create a file when that file already exists.
// 267 The directory name is invalid.
// 1314 A required privilege is not held by the client.
// 99999 winapi error #99999
// The system cannot find the file specified. | file exists | invalid argument | The system cannot find the path specified.
// not supported by windows
// FILE_NOT_FOUND true false false false
// PATH_NOT_FOUND true false false false
// ACCESS_DENIED false false true false
// ALREADY_EXISTS false true false false
// FILE_EXISTS false true false false
// DIR_NOT_EMPTY false true false false
// ENOENT true false false false
// EACCES false false true false
// EEXIST false true false false
// SHARING_VIOLATION false false false false
// CALL_NOT_IMPLEMENTED false false false true
// GetFileAttributesEx C:\gocpp-no-such-dir\file: The system cannot find the path specified. true true
// true GetFileAttributesEx true
// GetFileAttributesEx C:\Windows\gocpp-no-such-file: The system cannot find the file specified. true
// mkdir C:\Windows: Cannot create a file when that file already exists. true true
// remove C:\Windows\gocpp-no-such-file: The system cannot find the file specified.
