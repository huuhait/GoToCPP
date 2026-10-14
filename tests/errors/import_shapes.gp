// phase: 01

// Import paths whose shape is wrong for gocpp (Phase 01 README §2.3). They
// are load errors, so nothing is type-checked.
package main

import (
	"::std"             // ERROR "namespace ::std needs no import: use std.X directly, or rename it: import cstd \x22::std\x22"
	"./helper.cpp"      // ERROR "cannot import \x22./helper.cpp\x22: a C or C\+\+ file can't be imported \(import its header; gocpp compiles the C and C\+\+ files in the package folder\)"
	"./physics"         // ERROR "invalid import path \x22./physics\x22: a local header ends in .h, .hh, .hpp, .hxx, .h\+\+ or .inl"
	"./x.H"             // ERROR "invalid import path \x22./x.H\x22: a local header ends in"
	. "<vector>"        // ERROR "cannot dot-import a header"
	. "::std::chrono"   // ERROR "cannot dot-import a namespace"
	string "<string.h>" // ERROR "cannot import header as string - string is a predeclared Go name \(use another name, like cstring\)"
	C "<cmath>"         // ERROR "cannot import header as C - C is reserved for C's built-in types"
	init "<cstdio>"     // ERROR "cannot import header as init"
)

func main() {}
