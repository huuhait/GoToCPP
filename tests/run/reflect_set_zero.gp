// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type S struct {
	A int
	B string
	C []int
}

func main() {
	x := 5
	s := S{1, "b", []int{1}}
	p := &x
	m := map[int]int{1: 1}
	for _, ptr := range []any{&x, &s, &p, &m} {
		reflect.ValueOf(ptr).Elem().SetZero()
	}
	fmt.Println(x, s, p == nil, m == nil)
	a := [3]int{1, 2, 3}
	reflect.ValueOf(&a).Elem().SetZero()
	fmt.Println(a)
	sl := []int{1, 2, 3}
	reflect.ValueOf(sl).Index(1).SetZero()
	fmt.Println(sl)
	reflect.ValueOf(sl).Clear()
	fmt.Println(sl, len(sl))
	mm := map[string]int{"a": 1, "b": 2}
	reflect.ValueOf(mm).Clear()
	fmt.Println(len(mm), mm != nil)
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(x).SetZero()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(1).Clear()
	}()
}

// Output:
// 0 {0  []} true true
// [0 0 0]
// [1 0 3]
// [0 0 0] 3
// 0 true
// recovered: reflect: reflect.Value.SetZero using unaddressable value
// recovered: reflect: call of reflect.Value.Clear on int Value
