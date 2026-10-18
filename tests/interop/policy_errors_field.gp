// phase: 04

// The policy table of docs/spec/interop.md ("Types that can't be copied"), a struct field of a C++
// type with no default constructor: Go's zero value of the struct (var s S, new(S)) needs one of
// the field, so the struct can't be defined: a clear error at the field's line, also if the program
// only builds the struct from a literal.
// others-ok: 2 each field also fails to convert its default member initializer in the generated struct (gocpp's own follow-on errors, counted in the last output line)
package main

import (
	"fmt"

	"./objects.h"
)

type W struct {
	w objects.Widget // ERROR "C\+\+: this C\+\+ type has no default constructor, so a struct field of it has no zero value"
	n int
}

type G struct {
	g objects.Guard // ERROR "C\+\+: this C\+\+ type has no default constructor, so a struct field of it has no zero value"
}

func main() {
	s := &W{w: objects.Widget(3), n: 1}
	fmt.Println(int(s.w.get()), s.n)
}
