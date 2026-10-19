// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func add(a, b int) int { return a + b }

type Handler func(string) error

func main() {
	v := reflect.ValueOf(add)
	fmt.Println(v.Kind(), v.Type(), v.IsNil(), v.Type().NumIn(), v.Type().NumOut(), v.Pointer() != 0)
	var h Handler
	hv := reflect.ValueOf(h)
	fmt.Println(hv.Kind(), hv.Type(), hv.IsNil(), hv.Type().In(0), hv.Type().Out(0))
	var f func()
	fv := reflect.ValueOf(&f).Elem()
	fmt.Println(fv.IsNil())
	called := false
	fv.Set(reflect.ValueOf(func() { called = true }))
	f()
	fmt.Println(called, fv.IsNil())
	fv.Set(reflect.Zero(fv.Type()))
	fmt.Println(f == nil)
	cl := func(x int) int { return x + 1 }
	var g func(int) int = cl
	gv := reflect.ValueOf(&g).Elem()
	fmt.Println(gv.Interface().(func(int) int)(41))
	fmt.Println(reflect.ValueOf(Handler(nil)).Type() == reflect.TypeOf(h), reflect.TypeOf(h).String(), reflect.TypeOf(h).Kind())
	fmt.Println(reflect.TypeOf(h).AssignableTo(reflect.TypeOf(func(string) error { return nil })))
}

// Output:
// func func(int, int) int false 2 1 true
// func main.Handler true string error
// true
// true false
// true
// 42
// true main.Handler func
// true
