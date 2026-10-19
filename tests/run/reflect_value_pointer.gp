// phase: 09

package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

func main() {
	x, y := 1, 2
	px1, px2, py := reflect.ValueOf(&x), reflect.ValueOf(&x), reflect.ValueOf(&y)
	fmt.Println(px1.Pointer() == px2.Pointer(), px1.Pointer() == py.Pointer(), px1.Pointer() != 0)
	fmt.Println(px1.UnsafePointer() == unsafe.Pointer(&x), px1.UnsafePointer() == px2.UnsafePointer())
	var np *int
	fmt.Println(reflect.ValueOf(np).Pointer(), reflect.ValueOf(np).UnsafePointer() == nil)
	s := []int{1, 2, 3}
	sv := reflect.ValueOf(s)
	fmt.Println(sv.Pointer() == reflect.ValueOf(s[:2]).Pointer(), sv.Pointer() == reflect.ValueOf(s[1:]).Pointer())
	fmt.Println(sv.UnsafePointer() == unsafe.Pointer(&s[0]), reflect.ValueOf([]int(nil)).Pointer())
	m := map[int]int{1: 1}
	m2 := m
	fmt.Println(reflect.ValueOf(m).Pointer() == reflect.ValueOf(m2).Pointer(), reflect.ValueOf(map[int]int{}).Pointer() == reflect.ValueOf(m).Pointer())
	ch := make(chan int)
	fmt.Println(reflect.ValueOf(ch).Pointer() == reflect.ValueOf(ch).Pointer(), reflect.ValueOf(ch).Pointer() != 0)
	f := func() {}
	fmt.Println(reflect.ValueOf(f).Pointer() != 0, reflect.ValueOf((func())(nil)).Pointer())
	up := unsafe.Pointer(&x)
	fmt.Println(reflect.ValueOf(up).Pointer() == reflect.ValueOf(&x).Pointer(), reflect.ValueOf(up).UnsafePointer() == up)
	defer func() { fmt.Println("recovered:", recover()) }()
	reflect.ValueOf(1).Pointer()
}

// Output:
// true false true
// true true
// 0 true
// true false
// true 0
// true false
// true true
// true 0
// true true
// recovered: reflect: call of reflect.Value.Pointer on int Value
