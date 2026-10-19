// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[-1\]
// Stderr: (?m)^main\.\(\*Stack\[\.\.\.\]\)\.Pop\(\.\.\.\)$
// Stderr: (?m)^main\.Drain\[\.\.\.\]\(\.\.\.\)$
// Stderr: (?m)^main\.main\(\)$

// An unrecovered panic in methods of generic types: the frames are main.(*Stack[...]).Pop, main.Drain[...] (a generic function) and main.main.
package main

import "fmt"

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func (s *Stack[T]) Pop() T {
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v
}

func (s Stack[T]) Peek() T { return s.items[len(s.items)-1] }

func Drain[T any](s *Stack[T]) T {
	return s.Pop()
}

func main() {
	s := &Stack[string]{}
	s.Push("a")
	s.Push("b")
	fmt.Println(s.Peek(), s.Pop())
	fmt.Println(s.Pop())
	fmt.Println(Drain(s))
}

// Output:
// b b
// a
