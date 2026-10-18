// phase: 07

// A type parameter needs a constraint.
package main

func NoConstraint[T](x T) {} // ERROR "missing type constraint"

func main() {}
