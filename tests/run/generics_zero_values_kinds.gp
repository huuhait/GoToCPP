// phase: 08

// Zero values in generic code for every kind of type argument: var zero T, *new(T), a result that
// is never set, a missing map key, a receive from a closed channel, a failed comma-ok assertion,
// the zero value inside arrays and structs, and comparison with the zero value.
package main

import "fmt"

type Pair[A, B any] struct {
	A A
	B B
}

type Named int
type Iface interface{ M() }

func Zero[T any]() T {
	var z T
	return z
}

func New[T any]() T { return *new(T) }

func Unset[T any]() (r T, ok bool) { return }

func Missing[K comparable, V any](m map[K]V, k K) V { return m[k] }

func Closed[T any]() T {
	c := make(chan T)
	close(c)
	return <-c
}

func Assert[T any](v any) T {
	t, _ := v.(T)
	return t
}

func Arr[T any]() [3]T { return [3]T{} }

func InStruct[T any]() Pair[T, *T] { return Pair[T, *T]{} }

func IsZero[T comparable](v T) bool {
	var z T
	return v == z
}

func Fill[T any](n int) []T { return make([]T, n) }

func Slot[T any]() *T {
	p := new(T)
	return p
}

func show[T any](name string) {
	z := Zero[T]()
	fmt.Printf("%-14s %T | %v | %v | %v | %v | %v | %v\n", name, z, z, New[T](), Missing(map[int]T{}, 1), Closed[T](), Assert[T](nil), Arr[T]())
}

func main() {
	show[int]("int")
	show[int8]("int8")
	show[uint64]("uint64")
	show[float32]("float32")
	show[complex128]("complex128")
	show[bool]("bool")
	show[string]("string")
	show[Named]("Named")
	show[*int]("*int")
	show[[]int]("[]int")
	show[map[string]int]("map")
	show[chan int]("chan")
	show[func()]("func")
	show[error]("error")
	show[any]("any")
	show[Iface]("Iface")
	show[[2]string]("[2]string")
	show[struct{ X, Y int }]("anon struct")
	show[Pair[int, string]]("Pair")
	show[Pair[*int, []error]]("Pair 2")
	r, ok := Unset[Pair[string, int8]]()
	fmt.Println(r, ok)
	fmt.Println(IsZero(0), IsZero(""), IsZero(Named(3)), IsZero[any](nil), IsZero[any](0), IsZero(Pair[int, string]{}), IsZero(Pair[int, string]{B: "x"}), IsZero([2]int{}), IsZero[*int](nil))
	fmt.Println(InStruct[int](), InStruct[string]().B == nil)
	fmt.Println(Fill[string](2), len(Fill[Pair[int, int]](3)), Fill[*int](2), Fill[error](1), *Slot[float64](), *Slot[[2]bool](), (*Slot[Pair[int8, int8]]()).A)
	var np *Pair[int, int]
	fmt.Println(np == nil, Zero[*Pair[int, int]]() == np)
}

// Output:
// int            int | 0 | 0 | 0 | 0 | 0 | [0 0 0]
// int8           int8 | 0 | 0 | 0 | 0 | 0 | [0 0 0]
// uint64         uint64 | 0 | 0 | 0 | 0 | 0 | [0 0 0]
// float32        float32 | 0 | 0 | 0 | 0 | 0 | [0 0 0]
// complex128     complex128 | (0+0i) | (0+0i) | (0+0i) | (0+0i) | (0+0i) | [(0+0i) (0+0i) (0+0i)]
// bool           bool | false | false | false | false | false | [false false false]
// string         string |  |  |  |  |  | [  ]
// Named          main.Named | 0 | 0 | 0 | 0 | 0 | [0 0 0]
// *int           *int | <nil> | <nil> | <nil> | <nil> | <nil> | [<nil> <nil> <nil>]
// []int          []int | [] | [] | [] | [] | [] | [[] [] []]
// map            map[string]int | map[] | map[] | map[] | map[] | map[] | [map[] map[] map[]]
// chan           chan int | <nil> | <nil> | <nil> | <nil> | <nil> | [<nil> <nil> <nil>]
// func           func() | <nil> | <nil> | <nil> | <nil> | <nil> | [<nil> <nil> <nil>]
// error          <nil> | <nil> | <nil> | <nil> | <nil> | <nil> | [<nil> <nil> <nil>]
// any            <nil> | <nil> | <nil> | <nil> | <nil> | <nil> | [<nil> <nil> <nil>]
// Iface          <nil> | <nil> | <nil> | <nil> | <nil> | <nil> | [<nil> <nil> <nil>]
// [2]string      [2]string | [ ] | [ ] | [ ] | [ ] | [ ] | [[ ] [ ] [ ]]
// anon struct    struct { X int; Y int } | {0 0} | {0 0} | {0 0} | {0 0} | {0 0} | [{0 0} {0 0} {0 0}]
// Pair           main.Pair[int,string] | {0 } | {0 } | {0 } | {0 } | {0 } | [{0 } {0 } {0 }]
// Pair 2         main.Pair[*int,[]error] | {<nil> []} | {<nil> []} | {<nil> []} | {<nil> []} | {<nil> []} | [{<nil> []} {<nil> []} {<nil> []}]
// { 0} false
// true true false true false true false true true
// {0 <nil>} true
// [ ] 3 [<nil> <nil>] [<nil>] 0 [false false] 0
// true true
