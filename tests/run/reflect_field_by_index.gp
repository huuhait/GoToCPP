// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type A struct{ P, Q int }
type B struct {
	A
	R string
}
type Cc struct {
	X int
	B
}

func main() {
	t := reflect.TypeOf(Cc{})
	for _, idx := range [][]int{{0}, {1}, {1, 0}, {1, 1}, {1, 0, 0}, {1, 0, 1}} {
		f := t.FieldByIndex(idx)
		fmt.Println(idx, f.Name, f.Type, f.Index)
	}
	v := reflect.ValueOf(Cc{X: 1, B: B{A{2, 3}, "r"}})
	fmt.Println(v.FieldByIndex([]int{1, 0, 1}), v.FieldByIndex([]int{1, 1}), v.FieldByIndex([]int{0}))
	defer func() { fmt.Println("recovered:", recover()) }()
	t.FieldByIndex([]int{0, 0})
}

// Output:
// [0] X int [0]
// [1] B main.B [1]
// [1 0] A main.A [0]
// [1 1] R string [1]
// [1 0 0] P int [0]
// [1 0 1] Q int [1]
// 3 r 1
// recovered: reflect: Field of non-struct type int
