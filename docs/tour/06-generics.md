# Generics

Type parameters work as in Go 1.25: generic functions, generic types, constraints with type sets,
and `iter.Seq` for ranging over functions. gocpp turns each instance into ordinary C++ written
with its own types; you will not see C++ templates in the generated code.

<!-- program: 05 id=generics go -->
```go
package main

import (
	"cmp"
	"fmt"
	"iter"
	"slices"
)

type Number interface {
	~int | ~int64 | ~float64
}

// Sum works for any type whose underlying type is in the type set.
func Sum[T Number](xs ...T) T {
	var total T
	for _, x := range xs {
		total += x
	}
	return total
}

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

// A generic type with methods.
type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(x T) { s.items = append(s.items, x) }

func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	x := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return x, true
}

// All is an iterator: a function that can be ranged over.
func (s *Stack[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := len(s.items) - 1; i >= 0; i-- {
			if !yield(len(s.items)-1-i, s.items[i]) {
				return
			}
		}
	}
}

func main() {
	fmt.Println(Sum(1, 2, 3), Sum(1.5, 2.5))
	fmt.Println(Map([]int{1, 2, 3}, func(i int) string { return fmt.Sprint(i * i) }))

	var s Stack[string]
	s.Push("a")
	s.Push("b")
	s.Push("c")
	for i, x := range s.All() {
		fmt.Print(i, "=", x, " ")
	}
	fmt.Println()
	top, _ := s.Pop()
	fmt.Println(top, cmp.Compare(1, 2), slices.Max([]int{4, 9, 2}))
	fmt.Printf("%T\n", s)
}
```

<!-- output -->
```text
6 4
[1 4 9]
0=c 1=b 2=a 
c -1 9
main.Stack[string]
```

## What to notice

- Generic code is open from phase 05 on, ahead of its plan phase, so no switch is needed. The
  [feature matrix](../spec/feature-matrix.md) says "07 (works from 05)".
- `cmp`, `slices`, `maps` and `iter` are Go's own packages, ported by `tools/portstd`.
- Each instance (`Stack[string]`) is a separate struct in the C++, and `%T` prints Go's name for
  it. What generics cost to compile is in the [guide](../guides/generics-compile-time.md).
- A generic type declared inside a function is the one thing that Go has and gocpp does not;
  declare it at package level.

Next: [Goroutines and channels](07-goroutines.md).
