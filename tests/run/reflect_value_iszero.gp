// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type S struct {
	A int
	B string
	_ int
}
type N struct {
	In S
	P  *int
}

func main() {
	n := 1
	zero := 0.0
	negz := -zero
	vals := []any{
		false, true, 0, 1, int8(0), int64(-1), uint(0), uint8(1), uintptr(0), 0.0, 0.5, negz, float32(0), complex(0, 0), complex(0, 1), complex(negz, 0),
		"", "x", [2]int{}, [2]int{0, 1}, [0]int{}, S{}, S{A: 1}, S{B: "x"}, N{}, N{In: S{A: 1}}, N{P: &n}, []int(nil), []int{}, map[int]int(nil), map[int]int{},
		(*int)(nil), &n, (func())(nil), func() {}, (chan int)(nil), make(chan int), error(nil), [2][]int{}, [2][]int{nil, {}}, struct{}{},
	}
	for i, v := range vals {
		if v == nil {
			continue
		}
		fmt.Printf("%d %T %v\n", i, v, reflect.ValueOf(v).IsZero())
	}
	var e error
	holder := struct {
		E error
		A any
	}{}
	fmt.Println(reflect.ValueOf(holder).Field(0).IsZero(), reflect.ValueOf(holder).Field(1).IsZero(), e == nil)
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.Value{}.IsZero()
	}()
}

// Output:
// 0 bool true
// 1 bool false
// 2 int true
// 3 int false
// 4 int8 true
// 5 int64 false
// 6 uint true
// 7 uint8 false
// 8 uintptr true
// 9 float64 true
// 10 float64 false
// 11 float64 true
// 12 float32 true
// 13 complex128 true
// 14 complex128 false
// 15 complex128 true
// 16 string true
// 17 string false
// 18 [2]int true
// 19 [2]int false
// 20 [0]int true
// 21 main.S true
// 22 main.S false
// 23 main.S false
// 24 main.N true
// 25 main.N false
// 26 main.N false
// 27 []int true
// 28 []int false
// 29 map[int]int true
// 30 map[int]int false
// 31 *int true
// 32 *int false
// 33 func() true
// 34 func() false
// 35 chan int true
// 36 chan int false
// 38 [2][]int true
// 39 [2][]int false
// 40 struct {} true
// true true true
// recovered: reflect: call of reflect.Value.IsZero on zero Value
