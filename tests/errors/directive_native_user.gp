// phase: 01

// //gocpp:native belongs to the standard library: a program can't use it (ADR-027).

package main

//gocpp:native gp::errors::comparable // ERROR "//gocpp:native is only for the standard library"
func comparableType(err error) bool

func main() {}
