// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	s := []int{0, 1, 2, 3, 4, 5}
	v := reflect.ValueOf(s)
	a := v.Slice(1, 4)
	fmt.Println(a.Interface(), a.Len(), a.Cap(), a.CanSet(), a.Type())
	b := v.Slice3(1, 3, 4)
	fmt.Println(b.Interface(), b.Len(), b.Cap())
	a.Index(0).SetInt(100)
	fmt.Println(s)
	arr := [4]string{"a", "b", "c", "d"}
	av := reflect.ValueOf(&arr).Elem()
	as := av.Slice(1, 3)
	fmt.Println(as.Interface(), as.Type(), as.Cap())
	as.Index(0).SetString("B")
	fmt.Println(arr)
	str := reflect.ValueOf("hello world")
	fmt.Println(str.Slice(0, 5), str.Slice(6, 11), str.Slice(3, 3).Len(), str.Slice(0, 5).Type())
	type Names []string
	nv := reflect.ValueOf(Names{"x", "y", "z"})
	fmt.Println(nv.Slice(1, 3).Type(), nv.Slice(1, 3).Interface())
	for _, f := range []func(){
		func() { v.Slice(2, 1) }, func() { v.Slice(0, 7) }, func() { v.Slice(-1, 2) }, func() { v.Slice3(0, 2, 7) },
		func() { v.Slice3(0, 3, 2) }, func() { reflect.ValueOf(arr).Slice(0, 1) }, func() { reflect.ValueOf(1).Slice(0, 1) },
		func() { str.Slice(0, 20) }, func() { str.Slice3(0, 1, 2) },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// [1 2 3] 3 5 false []int
// [1 2] 2 3
// [0 100 2 3 4 5]
// [b c] []string 3
// [a B c d]
// hello world 0 string
// main.Names [y z]
// recovered: reflect.Value.Slice: slice index out of bounds
// recovered: reflect.Value.Slice: slice index out of bounds
// recovered: reflect.Value.Slice: slice index out of bounds
// recovered: reflect.Value.Slice3: slice index out of bounds
// recovered: reflect.Value.Slice3: slice index out of bounds
// recovered: reflect.Value.Slice: slice of unaddressable array
// recovered: reflect: call of reflect.Value.Slice on int Value
// recovered: reflect.Value.Slice: string slice index out of bounds
// recovered: reflect: call of reflect.Value.Slice3 on string Value
