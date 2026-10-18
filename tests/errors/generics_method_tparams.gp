// phase: 07

// Methods have no type parameters of their own.
package main

type Box struct{ v int }

func (b Box) Map[U any](f func(int) U) U { return f(b.v) } // ERROR "method must have no type parameters"

func main() {}
