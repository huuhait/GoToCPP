// phase: 08

// Zero values of type parameters with every kind of type argument.
package main

import "fmt"

type Pair struct {
	A int
	B string
}
type Fn func(int) int
type Named []int

func Zero[T any]() T {
	var z T
	return z
}

func IsZero[T comparable](x T) bool {
	var z T
	return x == z
}

func NewZero[T any]() T { return *new(T) }

func Reset[T any](p *T) { *p = Zero[T]() }

func ZeroSlice[T any](n int) []T { return make([]T, n) }

func Nilness[T any](x T) bool { return any(x) == nil }

func main() {
	fmt.Println(Zero[int](), Zero[float64](), Zero[string]() == "", Zero[bool](), Zero[uint8](), Zero[complex128]())
	fmt.Printf("%v %v %v %v %v\n", Zero[Pair](), Zero[[3]int](), Zero[*int]() == nil, Zero[error]() == nil, Zero[any]() == nil)
	fmt.Println(Zero[[]int]() == nil, Zero[map[string]int]() == nil, Zero[chan int]() == nil, Zero[Fn]() == nil, Zero[Named]() == nil)
	fmt.Println(IsZero(0), IsZero(1), IsZero(""), IsZero("x"), IsZero(Pair{}), IsZero(Pair{1, ""}), IsZero([2]bool{}), IsZero((*int)(nil)), IsZero[any](nil), IsZero[any](0))
	fmt.Println(NewZero[Pair](), NewZero[int](), len(NewZero[[2]Pair]()))
	p := Pair{3, "x"}
	Reset(&p)
	n := 5
	Reset(&n)
	fmt.Println(p, n, ZeroSlice[Pair](2), ZeroSlice[*int](1), ZeroSlice[error](2))
	fmt.Println(Nilness(0), Nilness[any](nil), Nilness[error](nil), Nilness[*int](nil), Nilness[any](Pair{}))
	fmt.Printf("%T %T %T %T\n", Zero[[]Pair](), Zero[map[int][]string](), Zero[func(int) string](), Zero[struct{ X, Y int }]())
}

// Output:
// 0 0 true false 0 (0+0i)
// {0 } [0 0 0] true true true
// true true true true true
// true false true false true false true true true false
// {0 } 0 2
// {0 } 0 [{0 } {0 }] [<nil>] [<nil> <nil>]
// false true true false false
// []main.Pair map[int][]string func(int) string struct { X int; Y int }
