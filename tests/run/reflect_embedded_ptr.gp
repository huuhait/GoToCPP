// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Inner struct{ N int }
type Outer struct {
	*Inner
	M int
}

func main() {
	o := Outer{Inner: &Inner{7}, M: 1}
	v := reflect.ValueOf(o)
	fmt.Println(v.FieldByName("N"), v.FieldByIndex([]int{0, 0}))
	t := reflect.TypeOf(o)
	f, ok := t.FieldByName("N")
	fmt.Println(f.Name, f.Index, ok, t.Field(0).Anonymous, t.Field(0).Type)
	nilO := Outer{}
	defer func() {
		fmt.Println("recovered:", recover())
	}()
	fmt.Println(reflect.ValueOf(nilO).FieldByIndex([]int{0, 0}))
}

// Output:
// 7 7
// N [0 0] true true *main.Inner
// recovered: reflect: indirection through nil pointer to embedded struct
