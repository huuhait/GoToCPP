// phase: 04

// C++ errors at the boundary are errors of the program (README §2.11, step 4.17): gocpp build
// reports them at the .gp line, after "C++: ", with the compiler's message.
package main

import (
	"<cstdio>"
	"<cstdlib>"
)

func format() string { return "%d\n" }

func main() {
	cstdio.printf(format(), 1) // ERROR "C\+\+: (cannot convert|no viable conversion from) 'gp::string' to 'const char ?\*'"
	_ = cstdlib.abs(1, 2)      // ERROR "C\+\+: too many arguments to function"
	cstdio.puts("ok")
}
