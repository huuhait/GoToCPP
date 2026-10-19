// phase: 10

// A macro on top of a var is one error, the syntax error (step 10.9).

package main

macro Note() {}

#Note() // ERROR "macro calls must be on top of a func or type declaration"
var x int

func main() { _ = x }
