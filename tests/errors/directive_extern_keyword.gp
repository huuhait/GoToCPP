// phase: 04

// An extern name is spelled as written in C++, so a C++ keyword is not a name of a function (ADR-128).

package main

//gocpp:extern C int // ERROR "//gocpp:extern: int is a C\+\+ keyword, so no C or C\+\+ function has that name"
func intFunction()

//gocpp:extern // ERROR "//gocpp:extern: new is a C\+\+ keyword"
func new()

//gocpp:extern gp::helper // ERROR "//gocpp:extern: gp::helper is in the namespace gp of gocpp's own code"
func helper()

func main() {}
