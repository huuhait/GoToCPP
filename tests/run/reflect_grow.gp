// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	var s []int
	v := reflect.ValueOf(&s).Elem()
	v.Grow(10)
	fmt.Println(len(s), cap(s) >= 10)
	s = append(s, 1, 2, 3)
	v.Grow(100)
	fmt.Println(len(s), cap(s) >= 103, s)
	c := cap(s)
	v.Grow(1)
	fmt.Println(cap(s) == c)
	t := []string{"a"}
	reflect.ValueOf(&t).Elem().Grow(5)
	fmt.Println(t, len(t), cap(t) >= 6)
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		v.Grow(-1)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(s).Grow(1)
	}()
}

// Output:
// 0 true
// 3 true [1 2 3]
// true
// [a] 1 true
// recovered: reflect.Value.Grow: negative len
// recovered: reflect: reflect.Value.Grow using unaddressable value
