// phase: 01

// <string.h> gets no package name (string is a predeclared Go name), so
// using it by its file name gets a hint.
package main

import "<string.h>"

func main() {
	_ = string.strlen(C.cstr("x")) // ERROR "<string.h> has no package name, because string is a predeclared Go name: give it a name: import cstring \x22<string.h>\x22"
}
