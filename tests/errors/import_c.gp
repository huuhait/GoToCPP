// phase: 01

// import "C" is cgo's; gocpp has no preamble and needs no import for C's
// built-in types.
package main

import "C" // ERROR "import \x22C\x22 is not allowed: C's built-in types need no import in gocpp, and there is no inline C preamble \(put C code in a header next to the .gp file\)"

func main() {}
