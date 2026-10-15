// phase: 04
// ExitCode: 2
// Stderr: (?m)^panic: runtime error: invalid memory address or nil pointer dereference$

// Members of C/C++ values go through gp::deref, as Go's x.m dereferences a pointer: a global
// object (globals.s_obj.x), a global raw pointer (globals.sp_set.x), and a nil one, which panics
// like Go's nil dereference.
package main

import (
	"fmt"

	"./globals.h"
)

func main() {
	fmt.Println(int(globals.s_obj.x), int(globals.sp_set.x), int(globals.sp_set.twice()))
	fmt.Println(int(globals.sp_nil.x))
}

// Output:
// 7 7 14
