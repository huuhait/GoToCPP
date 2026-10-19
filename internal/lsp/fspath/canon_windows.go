//go:build windows

package fspath

import "syscall"

// nativeLongName is GetLongPathName: the long form of an existing name, for a name with 8.3 short parts.
func nativeLongName(path string) (string, bool) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, 512)
	for {
		n, err := syscall.GetLongPathName(p, &buf[0], uint32(len(buf)))
		if err != nil || n == 0 {
			return "", false
		}
		if int(n) <= len(buf) {
			return syscall.UTF16ToString(buf[:n]), true
		}
		buf = make([]uint16, n)
	}
}
