// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./macro_runtime_clash\.gp:18:9: the macro Println of the imported headers has a name that gocpp's generated C\+\+ uses for something else too

// A macro that the program uses through its header package (pmacro.Println) and that collides
// with a name of the generated code (gp::fmtshim::Println of fmt.Println) can't be #undef'd: the
// build stops with an error at the macro's use.
package main

import (
	"fmt"

	"./pmacro.h"
)

func main() {
	fmt.Println("hi")
	pmacro.Println(3)
}

// Output:
