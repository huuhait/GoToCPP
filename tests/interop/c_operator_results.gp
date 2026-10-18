// phase: 04

// C++ operators can promote primitive operands or return an unrelated class type. The
// declared types of their operands do not establish the opaque result's type.
package main

import (
	"fmt"

	"./cgostr.h"
)

func main() {
	var c C.char = 1
	var d C.double = 0.5
	mixed := c + d
	fmt.Println(float64(mixed + 0.5))

	// sv_shout is an opaque C++ call that returns std::string; adding a char returns a
	// std::string too, not a C.char. Its result may compare with a string literal.
	s := cgostr.sv_shout("word") + C.char('!')
	fmt.Println(bool(s == "word!!"))

	p := C.CString("abcd") // a raw pointer from C: C++ operators work on it (a declared *C.char is a gocpp pointer)
	defer C.free(p)
	diff := p - p
	fmt.Println(bool(diff == 0), bool((p-p) == 0))

	// Unary + promotes char to int as well: 300 is a valid comparison operand there.
	promoted := +c
	fmt.Println(bool(promoted == 300))
}

// Output:
// 2
// true
// true true
// false
