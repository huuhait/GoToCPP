// phase: 01

// A conversion of an opaque value keeps its Go type (Phase 01 README §2.5
// step 5), so a mistake with the result is reported, as for a Go value.
package main

import (
	"fmt"

	cstring "<string.h>"
)

func main() {
	n := int(cstring.strlen(C.cstr("abc")))
	_ = n + "x"                             // ERROR "invalid operation: n \+ \x22x\x22 \(mismatched types int and untyped string\)"
	var s string = n                        // ERROR "cannot use n \(variable of type int\) as string value in variable declaration"
	m := float64(cstring.strlen(C.cstr("ab")))
	var k int = m // ERROR "cannot use m \(variable of type float64\) as int value in variable declaration"
	fmt.Println(s, k)
}
