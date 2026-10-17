// phase: 01

// A cgo preamble (the comment above import "C") is not supported either: put the C code in a
// header next to the .gp file and import it ("./helpers.h", step 4.16).
package main

/*
#include <stdio.h>
static void hello(void) { puts("hi"); }
*/
import "C" // ERROR "import \x22C\x22 is not allowed: C's built-in types need no import in gocpp, and there is no inline C preamble \(put C code in a header next to the .gp file\)"

func main() {
	C.hello()
}
