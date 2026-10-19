//go:build windows

package main

import (
	"path/filepath"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var procGetLongPathNameW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetLongPathNameW")

// canonPath returns the path p with the case of each existing element as the file system stores it
// (GetLongPathNameW, which also expands 8.3 names such as PROGRA~1) and a capital drive letter. A
// path that does not exist, or that the call refuses, comes back cleaned and with the drive letter
// fixed only.
func canonPath(p string) string {
	if p == "" {
		return p
	}
	p = filepath.Clean(p)
	if long, ok := longPath(p); ok {
		p = long
	}
	if len(p) >= 2 && p[1] == ':' && 'a' <= p[0] && p[0] <= 'z' {
		p = string(p[0]-'a'+'A') + p[1:]
	}
	return p
}

func longPath(p string) (string, bool) {
	src, err := syscall.UTF16FromString(p)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, 512)
	for range 2 {
		n, _, _ := procGetLongPathNameW.Call(uintptr(unsafe.Pointer(&src[0])), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		switch {
		case n == 0:
			return "", false
		case int(n) > len(buf):
			buf = make([]uint16, n)
		default:
			return string(utf16.Decode(buf[:n])), true
		}
	}
	return "", false
}
