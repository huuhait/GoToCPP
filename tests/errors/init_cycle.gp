// phase: 03

// A package variable's initializer can't refer to the variable itself, directly, through a
// function or through a func literal in another variable (go/types' errors, as in Go). gocpp's
// gp_var_init relies on this: a variable is still zero while its initializer runs, and nothing
// in the initializer can read it, so a large one is built in place.
package main

var a = a + 1 // ERROR "initialization cycle: a refers to itself"

var b = f() // ERROR "initialization cycle for b(.|\n)*b refers to f(.|\n)*f refers to b"

func f() int { return b }

var c = g() // ERROR "initialization cycle for c(.|\n)*c refers to g(.|\n)*g refers to h(.|\n)*h refers to c"

var h = func() int { return c }

func g() int { return h() }

func main() {}
