// phase: 04

// Two files of a package import headers that are fine alone and conflict when they are
// included together, as every C++ file of the package includes them: a.h is C++ and b.h is C,
// and both declare conflict_f. The error is at the import of b.h, in gocpp check and in the
// build.
package main

import "./b.h" // ERROR "header \x22./b.h\x22 doesn't compile with the headers of the other files of the package: b.h:1:5: .*linkage"

func main() {
	A()
	println(b.conflict_f(2))
}
