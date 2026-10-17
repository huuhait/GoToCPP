// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./cstr_result\.gp:14:9: C\.cstr\(s\) can be only an argument of a C/C\+\+ function or the value of a new variable \(name := C\.cstr\(s\)\): its C string lives only as long as that; for a C string that lives longer, use C\.CString and C\.free$

// The holder of C.cstr(s) lives to the end of its statement (or as long as its variable): as a
// result, its C string would dangle at once (step 4.9). C.CString makes a copy that lives until
// C.free.
package main

import "<cstdio>"

func name(s string) *C.char {
	cstdio.puts(C.cstr(s)) // fine: an argument
	return C.cstr(s)
}

func main() {
	cstdio.puts(name("x"))
}

// Output:
