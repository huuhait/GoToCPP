// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	s := make([]int, 2, 8)
	for i := range s {
		s[i] = i + 1
	}
	v := reflect.ValueOf(&s).Elem()
	v.SetLen(5)
	fmt.Println(len(s), cap(s), s)
	v.SetLen(1)
	fmt.Println(len(s), cap(s), s)
	v.SetCap(4)
	fmt.Println(len(s), cap(s))
	v.SetLen(4)
	fmt.Println(s)
	for _, f := range []func(){
		func() { v.SetLen(5) }, func() { v.SetLen(-1) }, func() { v.SetCap(9) }, func() { v.SetCap(2) },
		func() { reflect.ValueOf(s).SetLen(1) }, func() { reflect.ValueOf(&[2]int{}).Elem().SetLen(1) },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// 5 8 [1 2 0 0 0]
// 1 8 [1]
// 1 4
// [1 2 0 0]
// recovered: reflect: slice length out of range in SetLen
// recovered: reflect: slice length out of range in SetLen
// recovered: reflect: slice capacity out of range in SetCap
// recovered: reflect: slice capacity out of range in SetCap
// recovered: reflect: reflect.Value.SetLen using unaddressable value
// recovered: reflect: call of reflect.Value.SetLen on array Value
