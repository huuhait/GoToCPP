// phase: 04

// An extern function that no file defines is a link error at the .gp line that calls it, and the
// hint names the declaration (ADR-128). The signature has Go types only: the package uses no C name
// or header, and is still a package that declares C functions.
package main

//gocpp:extern C
func nobody_defines_this(x int32) int32

func main() {
	println(nobody_defines_this(1)) // ERROR "(?s)C\+\+: undefined reference to 'nobody_defines_this'.*nobody_defines_this is declared with //gocpp:extern C at .*extern_link_c.*main\.gp:9, and no file of the program defines the C function nobody_defines_this"
}
