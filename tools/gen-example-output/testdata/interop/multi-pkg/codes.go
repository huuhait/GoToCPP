// The cgo version of examples/multi-pkg/codes/codes.gp, for check.sh: with
// it, real Go runs the multi-pkg example and its C file (luhn.c).

// Package codes makes item codes that end in a check digit. The check digit
// is computed in C: luhn.c is compiled and linked with this package.
package codes

/*
#include <stdlib.h>
#include "luhn.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// cstr stands for gocpp's C.cstr(s).
func cstr(s string) (*C.char, func()) {
	cs := C.CString(s)
	return cs, func() { C.free(unsafe.Pointer(cs)) }
}

func init() { fmt.Println("init codes") }

// Make returns base followed by its Luhn check digit.
func Make(base string) (string, error) {
	cs, free := cstr(base)
	defer free()
	d := int(C.luhn_check_digit(cs))
	if d < 0 {
		return "", fmt.Errorf("codes: %s is not a number", base)
	}
	return base + string(rune('0'+d)), nil
}

// Valid reports whether code ends in the right check digit.
func Valid(code string) bool {
	cs, free := cstr(code)
	defer free()
	return C.luhn_valid(cs) != 0
}
