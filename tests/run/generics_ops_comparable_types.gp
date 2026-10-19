// phase: 08

// == on comparable type parameters: structs, arrays, pointers, interfaces, channels, NaN.
package main

import "fmt"

type P struct {
	X, Y int
	S    string
}
type W struct {
	P P
	F float64
	A [2]int8
}
type I interface{ M() }
type T1 int

func (T1) M() {}

type T2 string

func (T2) M() {}

func Eq[T comparable](a, b T) bool { return a == b }
func Ne[T comparable](a, b T) bool { return a != b }
func Index[S ~[]E, E comparable](s S, x E) int {
	for i, v := range s {
		if v == x {
			return i
		}
	}
	return -1
}
func Dedup[E comparable](s []E) []E {
	seen := map[E]bool{}
	var out []E
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func zero() float64 { return 0 }
func nan() float64  { z := zero(); return z / z }
func inf() float64  { return 1 / zero() }
func negz() float64 { return -zero() }

func main() {
	fmt.Println(Eq(1, 1), Eq("a", "b"), Eq(P{1, 2, "x"}, P{1, 2, "x"}), Ne(P{1, 2, "x"}, P{1, 2, "y"}), Eq([3]int{1, 2, 3}, [3]int{1, 2, 3}))
	fmt.Println(Eq(W{F: 1}, W{F: 1}), Eq(W{F: nan()}, W{F: nan()}), Eq(nan(), nan()), Ne(nan(), nan()), Eq([2]float64{nan()}, [2]float64{nan()}))
	x, y := 1, 1
	fmt.Println(Eq(&x, &x), Eq(&x, &y), Eq[*int](nil, nil), Eq(make(chan int), make(chan int)))
	c := make(chan int)
	fmt.Println(Eq(c, c), Eq[I](T1(1), T1(1)), Eq[I](T1(1), T2("1")), Eq[I](nil, nil), Eq[I](nil, T1(0)), Eq[any](1, 1.0), Eq[any]("a", "a"), Eq[any](int8(1), int16(1)))
	fmt.Println(Index([]string{"a", "b"}, "b"), Index([]P{{1, 2, "a"}}, P{1, 2, "a"}), Index([]any{1, "x", 2.5}, any("x")), Index([]I{T1(1), T2("a")}, I(T2("a"))))
	fmt.Println(Dedup([]int{1, 2, 1, 3, 2}), Dedup([]P{{1, 1, ""}, {1, 1, ""}}), Dedup([]any{1, "a", 1, "a", 2.5, 2.5, nil, nil}), Dedup([]bool{true, true}))
}

// Output:
// true false true true true
// true false false true false
// true false true false
// true true false true false false true false
// 1 0 1 1
// [1 2 3] [{1 1 }] [1 a 2.5 <nil>] [true]
