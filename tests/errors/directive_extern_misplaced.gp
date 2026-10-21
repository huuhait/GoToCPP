// phase: 04

// //gocpp:extern is for the doc comment of a function without a body (ADR-128).

package main

//gocpp:extern // ERROR "//gocpp:extern must be in the doc comment of a function"
var x int

//gocpp:extern C ns::f // ERROR "//gocpp:extern C: ns::f is not the name of a C function"
func a()

//gocpp:extern C one two // ERROR "usage: //gocpp:extern \[C\|C\+\+\] \[name\]"
func b()

//gocpp:extern // ERROR "//gocpp:extern is for a function without a body"
func c() {}

type T int

//gocpp:extern // ERROR "//gocpp:extern is for a function, not a method"
func (T) m()

//gocpp:extern // ERROR "//gocpp:extern is for a function that is not generic"
func d[E any](e E)

//gocpp:externs f // ERROR "//gocpp: directives must come before the package clause"
func e()

func main() {}
