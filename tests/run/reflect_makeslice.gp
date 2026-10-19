// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }
type IDs []int

func main() {
	v := reflect.MakeSlice(reflect.TypeOf([]int{}), 3, 10)
	fmt.Println(v.Len(), v.Cap(), v.Interface(), v.CanSet(), v.Type())
	v.Index(1).SetInt(5)
	fmt.Println(v.Interface())
	ps := reflect.MakeSlice(reflect.TypeOf([]Pt{}), 2, 2)
	ps.Index(0).Field(1).SetInt(9)
	fmt.Println(ps.Interface())
	ss := reflect.MakeSlice(reflect.TypeOf([]string{}), 2, 2)
	ss.Index(0).SetString("hi")
	fmt.Println(ss.Interface(), len(ss.Index(1).String()))
	ids := reflect.MakeSlice(reflect.TypeOf(IDs{}), 1, 1)
	fmt.Println(ids.Type(), ids.Interface())
	nested := reflect.MakeSlice(reflect.TypeOf([][]int{}), 2, 2)
	nested.Index(0).Set(reflect.ValueOf([]int{1, 2}))
	fmt.Println(nested.Interface(), nested.Index(1).IsNil())
	e := reflect.MakeSlice(reflect.TypeOf([]int{}), 0, 0)
	fmt.Println(e.IsNil(), e.Len(), e.Interface() != nil)
	ms := reflect.MakeSlice(reflect.TypeOf([]map[string]int{}), 1, 1)
	fmt.Println(ms.Index(0).IsNil())
	for _, f := range []func(){
		func() { reflect.MakeSlice(reflect.TypeOf(1), 1, 1) },
		func() { reflect.MakeSlice(reflect.TypeOf([]int{}), -1, 1) },
		func() { reflect.MakeSlice(reflect.TypeOf([]int{}), 1, -1) },
		func() { reflect.MakeSlice(reflect.TypeOf([]int{}), 2, 1) },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// 3 10 [0 0 0] false []int
// [0 5 0]
// [{0 9} {0 0}]
// [hi ] 0
// main.IDs [0]
// [[1 2] []] true
// false 0 true
// true
// recovered: reflect.MakeSlice of non-slice type
// recovered: reflect.MakeSlice: negative len
// recovered: reflect.MakeSlice: negative cap
// recovered: reflect.MakeSlice: len > cap
