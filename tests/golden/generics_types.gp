// phase: 07

// Instances of generic types: one struct, descriptor and method set for each list of type arguments, named gp_inst_<hash>_<Name> in the namespace of the package that declares the type, written in the header with the methods inline.
package main

import "fmt"

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func (s Stack[T]) Len() int { return len(s.items) }

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

type Lenner interface{ Len() int }

func main() {
	var s Stack[int]
	s.Push(1)
	p := Pair[string, Stack[int]]{"k", s}
	var l Lenner = &s
	fmt.Printf("%v %v %T\n", p, l.Len(), p)
}
