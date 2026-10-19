// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type S struct {
	A int
	B []int
	c int
}

func main() {
	s := S{1, []int{1, 2}, 3}
	v := reflect.ValueOf(&s).Elem()
	a := v.Field(0)
	fmt.Println(a.CanAddr(), a.Addr().Type(), a.Addr().Elem().Int())
	a.Addr().Elem().SetInt(10)
	fmt.Println(s.A)
	*(a.Addr().Interface().(*int)) = 20
	fmt.Println(s.A)
	e := v.Field(1).Index(1)
	fmt.Println(e.CanAddr(), e.Addr().Type())
	*(e.Addr().Interface().(*int)) = 99
	fmt.Println(s.B)
	fmt.Println(v.Addr().Interface().(*S) == &s, v.Addr().Type())
	fmt.Println(v.Field(2).CanAddr(), v.Field(2).Addr().CanInterface())
	fmt.Println(reflect.ValueOf(s).CanAddr(), reflect.ValueOf(&s).CanAddr(), reflect.ValueOf(&s).Elem().CanAddr())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(s).Field(0).Addr()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(1).Addr()
	}()
	arr := [2]int{1, 2}
	av := reflect.ValueOf(&arr).Elem()
	ptr := av.Index(0).Addr().Interface().(*int)
	*ptr = 100
	fmt.Println(arr)
}

// Output:
// true *int 1
// 10
// 20
// true *int
// [1 99]
// true *main.S
// true false
// false false true
// recovered: reflect.Value.Addr of unaddressable value
// recovered: reflect.Value.Addr of unaddressable value
// [100 2]
