// phase: 07

// Type parameters that no parameter mentions need explicit arguments, and each choice is its own instance.
package main

import "fmt"

func Tag[T any]() string { return fmt.Sprintf("%T", *new(T)) }

func Size[T any, N ~int](n N) []T { return make([]T, n) }

func Ignore[T, U any](v T) T { return v }

func Cast[To, From ~int | ~float64](f From) To { return To(f) }

type Meters int

func main() {
	fmt.Println(Tag[int](), Tag[string](), Tag[[]Meters](), Tag[map[string]*int](), Tag[error](), Tag[any]())
	fmt.Println(Size[string](2), len(Size[bool](3)), Size[float64, Meters](1))
	fmt.Println(Ignore[int, string](1), Ignore[int, bool](2), Ignore[string, int]("s"))
	fmt.Println(Cast[int](2.9), Cast[float64](3), Cast[Meters](7.5), Cast[int, Meters](Meters(4)))
}

// Output:
// int string []main.Meters map[string]*int <nil> <nil>
// [ ] 3 [0]
// 1 2 s
// 2 3 7 4
