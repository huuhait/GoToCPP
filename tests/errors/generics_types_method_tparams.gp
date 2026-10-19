// phase: 07

// A method of a generic type has no type parameters of its own.
package main

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Map[U any](f func(T) U) []U { return nil } // ERROR "method must have no type parameters"

func main() {}
