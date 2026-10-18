// phase: 01

// //gocpp:native is for the doc comment of a function without a body; a misspelled name is an unknown directive.

package main

//gocpp:native gp::f // ERROR "//gocpp:native must be in the doc comment of a function" "//gocpp:native is only for the standard library"
var x int

//gocpp:nativ gp::f // ERROR "//gocpp: directives must come before the package clause"
func g()

func main() {}
