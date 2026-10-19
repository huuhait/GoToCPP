// phase: 14

// C.wstr and C.GoStringW (step 14.4, ADR-082) with real W functions of kernel32: text goes to Windows as UTF-16
// and comes back as the same Go string, also with a lone surrogate (WTF-8) and with non-BMP characters.

//gocpp:link kernel32

package main

import (
	"fmt"

	"<windows.h>"
)

// roundTrip stores s in an environment variable and reads it back.
func roundTrip(s string) string {
	name := C.wstr("GOCPP_WSTR_TEST")
	if windows.SetEnvironmentVariableW(name, C.wstr(s)) == 0 {
		return fmt.Sprint("SetEnvironmentVariableW failed: ", int(windows.GetLastError()))
	}
	var buf [64]windows.WCHAR
	n := windows.GetEnvironmentVariableW(name, &buf[0], 64)
	got := C.GoStringW(&buf[0])
	if gotN := C.GoStringWN(&buf[0], C.int(n)); gotN != got {
		return "GoStringW and GoStringWN differ: " + gotN
	}
	windows.SetEnvironmentVariableW(name, nil)
	return got
}

func main() {
	for _, s := range []string{
		"plain",
		"héllo wörld",
		"世界",
		"\U0001F642 smile",
		"tab\tand 'quotes'",
	} {
		got := roundTrip(s)
		fmt.Printf("%q round trip %v\n", s, got == s)
	}

	// Length in UTF-16 units, counted by Windows: a character beyond U+FFFF is two units.
	fmt.Println("lstrlenW:", int(windows.lstrlenW(C.wstr("abc"))), int(windows.lstrlenW(C.wstr("é世"))),
		int(windows.lstrlenW(C.wstr("\U0001F642"))), int(windows.lstrlenW(C.wstr(""))))

	// A NUL inside the string ends the C string (as for C.cstr).
	fmt.Println("NUL:", int(windows.lstrlenW(C.wstr("ab\x00cd"))))

	// An invalid UTF-8 byte becomes U+FFFD, as in Go's syscall.UTF16FromString.
	fmt.Printf("invalid: %q\n", roundTrip("a\xffz"))

	// A lone surrogate written as three bytes (WTF-8) stays a lone UTF-16 unit and comes back as the
	// same three bytes. Windows file names can hold one.
	lone := "x\xed\xa0\x80y"
	fmt.Println("lone surrogate round trip:", roundTrip(lone) == lone)

	// Ordinal comparison by Windows of two wide strings made by C.wstr.
	a, b := C.wstr("apple"), C.wstr("banana")
	fmt.Println("CompareStringOrdinal:", int(windows.CompareStringOrdinal(a, -1, b, -1, 0)) == int(windows.CSTR_LESS_THAN))

	// A holder in a variable lives as long as the variable.
	title := C.wstr("kept")
	fmt.Println("variable:", int(windows.lstrlenW(title)))

	// A nil pointer is the empty string.
	fmt.Printf("nil: %q\n", C.GoStringW(nil))

	// FormatMessageW fills a buffer with text of Windows; the message of an error code, in US English.
	var msg [256]windows.WCHAR
	n := windows.FormatMessageW(windows.FORMAT_MESSAGE_FROM_SYSTEM|windows.FORMAT_MESSAGE_IGNORE_INSERTS, nil,
		windows.ERROR_FILE_NOT_FOUND, 0x0409, &msg[0], 256, nil)
	fmt.Printf("FormatMessageW: %d %q\n", int(n), C.GoStringWN(&msg[0], C.int(n)))
}
