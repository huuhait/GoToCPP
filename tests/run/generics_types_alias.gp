// phase: 07

// Aliases of instances and generic aliases (Go 1.24): the same type under another name.
package main

import "fmt"

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

type IntStack = Stack[int]

type Pair[A, B any] struct {
	First  A
	Second B
}

type SP[A any] = Pair[string, A]

type Vec[T any] = []T

type Set[T comparable] = map[T]struct{}

func Sum(v Vec[int]) int {
	s := 0
	for _, x := range v {
		s += x
	}
	return s
}

func Mk[T comparable](xs ...T) Set[T] {
	s := Set[T]{}
	for _, x := range xs {
		s[x] = struct{}{}
	}
	return s
}

func main() {
	var s IntStack
	s.Push(1)
	var t *Stack[int] = &s
	t.Push(2)
	fmt.Println(s.items)
	fmt.Printf("%T %T\n", s, t)
	p := SP[float64]{"x", 2.5}
	fmt.Printf("%T %v\n", p, p)
	fmt.Println(Sum(Vec[int]{1, 2, 3}), len(Mk("a", "b", "a")))
	var v Vec[string] = []string{"q"}
	fmt.Printf("%T %v\n", v, v)
	var a any = IntStack{}
	_, ok := a.(Stack[int])
	fmt.Println(ok)
}

// Output:
// [1 2]
// main.Stack[int] *main.Stack[int]
// main.Pair[string,float64] {x 2.5}
// 6 2
// []string [q]
// true
