// phase: 05

// A name with ! in the declaration of a type (step 10.13): the parser reads `type T [` as the start of a
// type parameter list, where only a called function or method can have a !, so the error comes from the
// parser, with the place named (a parameter's array length is a call the checker cannot place:
// must_places.gp).

package main

func two(a int) (int, error) { return a * 2, nil }

type Table [two!(2)]string // ERROR "cannot use two! here: a name with ! can only be a called function or method, or a func declaration"

func main() {}
