// phase: 04

// Mistakes in the use of a helper header are C++ errors at the .gp line, in a package folder
// that is built as "." (step 4.16). They come before the link, so shelf.h needs no .cpp.
package main

import "./shelf.h"

func main() {
	println(shelf.shelf_add(1, 2))
	println(shelf.shelf_add(1)) // ERROR "C\+\+: (too few arguments to function|no matching function for call to 'shelf_add')"
	println(shelf.shelf_sub("a", 2)) // ERROR "C\+\+: (invalid conversion from 'const char ?\*' to 'int'|cannot initialize a parameter of type 'int')"
}
