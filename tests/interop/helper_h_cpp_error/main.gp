// phase: 04

// The trap of a .h + .cpp pair (step 4.16): mathlib.h parses as C, so gocpp includes it in
// extern "C" for the generated code, but mathlib.cpp includes it as C++ and defines add with C++
// linkage. The link error is at the .gp line, and its hint names the cause and both fixes.
// (The regexp has escapes, so it can't match its own echo in the source line of the message.)
package main

import "./mathlib.h"

func main() {
	println(mathlib.add(1, 2)) // ERROR "(?s)C\+\+: undefined reference to 'add'.*\(\./mathlib\.h parses as C.*mathlib\.cpp includes it outside extern.*add //gocpp:no-extern-c \./mathlib\.h, or #ifdef __cplusplus extern"
}
