// phase: 09

package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

type T struct{ A, B int32 }

func main() {
	x := T{1, 2}
	up := unsafe.Pointer(&x)
	v := reflect.ValueOf(up)
	fmt.Println(v.Kind(), v.Type(), v.IsNil(), v.Pointer() != 0, v.UnsafePointer() == up)
	var nup unsafe.Pointer
	nv := reflect.ValueOf(nup)
	fmt.Println(nv.Kind(), nv.IsNil(), nv.Pointer(), nv.UnsafePointer() == nil)
	type H struct{ P unsafe.Pointer }
	h := H{}
	hv := reflect.ValueOf(&h).Elem()
	hv.Field(0).Set(v)
	fmt.Println(h.P == up, (*T)(h.P).B)
	hv.Field(0).SetZero()
	fmt.Println(h.P == nil)
	fmt.Println(reflect.TypeOf(up).Name(), reflect.TypeOf(up).PkgPath(), reflect.TypeOf(up).String(), reflect.TypeOf(up).Comparable())
	fmt.Println(reflect.ValueOf(up).Interface().(unsafe.Pointer) == up)
	fmt.Println(reflect.ValueOf(uintptr(up)).Kind(), reflect.ValueOf(uintptr(up)).Uint() == uint64(uintptr(up)))
}

// Output:
// unsafe.Pointer unsafe.Pointer false true true
// unsafe.Pointer true 0 true
// true 2
// true
// Pointer unsafe unsafe.Pointer true
// true
// uintptr true
