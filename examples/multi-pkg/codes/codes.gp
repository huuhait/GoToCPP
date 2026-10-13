// Package codes makes item codes that end in a check digit. The check digit
// is computed in C: luhn.c is compiled and linked with this package.
package codes

import (
	"fmt"

	"./luhn.h"
)

func init() { fmt.Println("init codes") }

// Make returns base followed by its Luhn check digit.
func Make(base string) (string, error) {
	d := int(luhn.luhn_check_digit(C.cstr(base)))
	if d < 0 {
		return "", fmt.Errorf("codes: %s is not a number", base)
	}
	return base + string(rune('0'+d)), nil
}

// Valid reports whether code ends in the right check digit.
func Valid(code string) bool {
	return luhn.luhn_valid(C.cstr(code)) != 0
}
