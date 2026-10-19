//go:build windows

package winpath

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetACP              = kernel32.NewProc("GetACP")
	procWideCharToMultiByte = kernel32.NewProc("WideCharToMultiByte")
	procGetShortPathNameW   = kernel32.NewProc("GetShortPathNameW")
)

const (
	cpUTF8          = 65001
	wcNoBestFitChar = 0x400
)

func acpSafe(s string) bool {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return true
	}
	if acp, _, _ := procGetACP.Call(); acp == cpUTF8 {
		return true // the system reads arguments as UTF-8
	}
	w := utf16.Encode([]rune(s))
	if len(w) == 0 {
		return true
	}
	var used int32
	// WC_NO_BEST_FIT_CHARS: a character without an exact counterpart is a failure, not a lookalike.
	n, _, _ := procWideCharToMultiByte.Call(0, wcNoBestFitChar, uintptr(unsafe.Pointer(&w[0])), uintptr(len(w)), 0, 0, 0, uintptr(unsafe.Pointer(&used)))
	return n != 0 && used == 0
}

func short(p string) (string, bool) {
	src, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, 512)
	for range 2 {
		n, _, _ := procGetShortPathNameW.Call(uintptr(unsafe.Pointer(src)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		switch {
		case n == 0:
			return "", false
		case int(n) > len(buf):
			buf = make([]uint16, n)
		default:
			s := string(utf16.Decode(buf[:n]))
			return s, acpSafe(s)
		}
	}
	return "", false
}
