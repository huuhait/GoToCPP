// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }

func main() {
	mt := reflect.TypeOf(map[string]int{})
	m := reflect.MakeMap(mt)
	m.SetMapIndex(reflect.ValueOf("a"), reflect.ValueOf(1))
	m.SetMapIndex(reflect.ValueOf("b"), reflect.ValueOf(2))
	fmt.Println(m.Len(), m.Interface(), m.IsNil(), m.Kind(), m.Type())
	real := m.Interface().(map[string]int)
	real["c"] = 3
	fmt.Println(m.Len(), m.MapIndex(reflect.ValueOf("c")))
	m2 := reflect.MakeMapWithSize(reflect.TypeOf(map[Pt][]string{}), 10)
	m2.SetMapIndex(reflect.ValueOf(Pt{1, 2}), reflect.ValueOf([]string{"x"}))
	fmt.Println(m2.Interface())
	m3 := reflect.MakeMap(reflect.TypeOf(map[any]bool{}))
	m3.SetMapIndex(reflect.ValueOf(1), reflect.ValueOf(true))
	fmt.Println(m3.Interface())
	z := reflect.Zero(mt)
	fmt.Println(z.IsNil(), z.Len())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.MakeMap(reflect.TypeOf(1))
	}()
}

// Output:
// 2 map[a:1 b:2] false map map[string]int
// 3 3
// map[{1 2}:[x]]
// map[1:true]
// true 0
// recovered: reflect.MakeMapWithSize of non-map type
