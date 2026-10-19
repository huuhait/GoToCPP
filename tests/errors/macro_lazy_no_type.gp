// phase: 05

// A lazy parameter with no type (step 10.13): the parser's error is the only one; the check of the lazy
// types does not add a second error about the broken type.

package main

macro M(a lazy int, b lazy) {} // ERROR "expected type, found '\)'"

func main() {}
