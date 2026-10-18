// phase: 07

// Zero values of type parameters: var z T, *new(T), return values on failure, for many kinds of types.
package main

import "fmt"

type Point struct{ X, Y int }
type Named struct {
	P    Point
	Name string
	Tags []string
}
type Celsius float64
type Handler func(int) int

func Zero[T any]() T {
	var z T
	return z
}

func Zero2[T any]() T { return *new(T) }

func Is[T comparable](v T) bool {
	var z T
	return v == z
}

func Lookup[K comparable, V any](m map[K]V, k K) (V, bool) {
	if v, ok := m[k]; ok {
		return v, true
	}
	var zero V
	return zero, false
}

func Reset[T any](p *T) { *p = *new(T) }

func Pop[T any](xs []T) (T, []T) {
	var zero T
	if len(xs) == 0 {
		return zero, xs
	}
	return xs[len(xs)-1], xs[:len(xs)-1]
}

func main() {
	fmt.Println(Zero[int](), Zero[uint8](), Zero[float32](), Zero[bool](), Zero[string]() == "", Zero[rune](), Zero[complex128]())
	fmt.Println(Zero[Point](), Zero[Celsius](), Zero[[3]int](), Zero[[0]int](), Zero[struct{}](), Zero[Named]())
	fmt.Println(Zero[*int]() == nil, Zero[[]int]() == nil, Zero[map[string]int]() == nil)
	fmt.Println(Zero[Handler]() == nil, Zero[func()]() == nil, Zero[error]() == nil, Zero[any]() == nil, Zero[fmt.Stringer]() == nil)
	fmt.Println(Zero2[int](), Zero2[string]() == "", Zero2[Point](), Zero2[*Point]() == nil, Zero2[[]string]() == nil)
	fmt.Println(Is(0), Is(""), Is(Point{}), Is(Point{1, 0}), Is[*int](nil), Is(0.0), Is([2]bool{}), Is(Celsius(1)))
	fmt.Println(Lookup(map[string]int{"a": 1}, "a"))
	fmt.Println(Lookup(map[string]Point{}, "x"))
	fmt.Println(Lookup(map[int][]string{}, 1))
	p := Point{3, 4}
	Reset(&p)
	n := Named{Point{1, 1}, "n", []string{"t"}}
	Reset(&n)
	s := "x"
	Reset(&s)
	fmt.Println(p, n.Name == "", n.Tags == nil, s == "")
	fmt.Println(Pop([]int{1, 2, 3}))
	fmt.Println(Pop([]string(nil)))
	fmt.Println(Pop([]Point{{1, 2}}))
}

// Output:
// 0 0 0 false true 0 (0+0i)
// {0 0} 0 [0 0 0] [] {} {{0 0}  []}
// true true true
// true true true true true
// 0 true {0 0} true true
// true true true false true true true false
// 1 true
// {0 0} false
// [] false
// {0 0} true true true
// 3 [1 2]
//  []
// {1 2} []
