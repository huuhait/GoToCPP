// phase: 04

// A header that ends in the middle of a declaration is an error at its import, in gocpp check
// and in the build: the build includes the header in front of other code, so the declaration
// would swallow it.
package main

import "./cut.h" // ERROR "header \x22./cut.h\x22 ends inside a declaration or a block that it doesn't close"

func main() {
	println(cut.cut_add(1, 2))
}
