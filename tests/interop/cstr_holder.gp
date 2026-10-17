// phase: 04

// C.cstr(s) gives a gocpp string to C as a const char* (step 4.9): a holder that lives to the end
// of the statement, or as long as the variable it is in. Through printf's `...` it is the holder's
// pointer (gp::cstr(s).gp_ptr()), and so is a holder variable. A string constant is a C literal
// for a C function, also a typed one.
package main

import (
	"<cstdio>"
)

const typed string = "a typed constant"

type label string

const named label = "a named constant"

var order string

func mk(s string) string {
	order += "m"
	return s + "!"
}

func f() C.int {
	order += "f"
	return 7
}

func main() {
	s := "gocpp"
	cstdio.puts(C.cstr(s))
	cstdio.printf("%s and %s and %s\n", C.cstr(s), C.cstr(s+"!"), C.cstr("lit"))
	cstdio.printf("%d %s\n", C.int(len(s)), C.cstr(s[1:3]))
	name := C.cstr(s + " holder")
	cstdio.printf("[%s]\n", name)
	cstdio.puts(name)
	copy := name
	var copy2 = copy
	name = C.cstr("another")
	cstdio.printf("copies: [%s] [%s]\n", copy, copy2)
	cstdio.puts(name)
	var other = C.cstr("")
	cstdio.printf("empty: [%s]\n", other)
	cstdio.puts(typed)
	cstdio.printf("%s, %s\n", typed, named)
	// The later side-effecting call forces the first holder into an evaluation-order
	// temporary. It must hold the owner, with .gp_ptr() only at printf's argument use.
	cstdio.printf("%s %d %s\n", C.cstr(mk(s)), f(), C.cstr(mk("x")+s))
	cstdio.puts(C.cstr(order))
	cstdio.fflush(cstdio.stdout)
}

// Output:
// gocpp
// gocpp and gocpp! and lit
// 5 oc
// [gocpp holder]
// gocpp holder
// copies: [gocpp holder] [gocpp holder]
// another
// empty: []
// a typed constant
// a typed constant, a named constant
// gocpp! 7 x!gocpp
// mfm
