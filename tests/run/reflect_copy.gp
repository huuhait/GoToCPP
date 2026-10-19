// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	dst := make([]int, 3)
	n := reflect.Copy(reflect.ValueOf(dst), reflect.ValueOf([]int{1, 2, 3, 4, 5}))
	fmt.Println(n, dst)
	n = reflect.Copy(reflect.ValueOf(dst), reflect.ValueOf([]int{9}))
	fmt.Println(n, dst)
	var arr [4]string
	n = reflect.Copy(reflect.ValueOf(&arr).Elem(), reflect.ValueOf([]string{"a", "b"}))
	fmt.Println(n, arr)
	src := [3]int{7, 8, 9}
	n = reflect.Copy(reflect.ValueOf(dst), reflect.ValueOf(src))
	fmt.Println(n, dst)
	b := make([]byte, 4)
	n = reflect.Copy(reflect.ValueOf(b), reflect.ValueOf("hello"))
	fmt.Println(n, string(b))
	// overlapping
	o := []int{1, 2, 3, 4, 5}
	reflect.Copy(reflect.ValueOf(o[1:]), reflect.ValueOf(o))
	fmt.Println(o)
	o2 := []int{1, 2, 3, 4, 5}
	reflect.Copy(reflect.ValueOf(o2), reflect.ValueOf(o2[2:]))
	fmt.Println(o2)
	ss := [][]int{{1}, {2}}
	ds := make([][]int, 2)
	reflect.Copy(reflect.ValueOf(ds), reflect.ValueOf(ss))
	ss[0][0] = 100
	fmt.Println(ds)
	for _, f := range []func(){
		func() { reflect.Copy(reflect.ValueOf(1), reflect.ValueOf([]int{})) },
		func() { reflect.Copy(reflect.ValueOf(dst), reflect.ValueOf(1)) },
		func() { reflect.Copy(reflect.ValueOf(dst), reflect.ValueOf([]string{})) },
		func() { reflect.Copy(reflect.ValueOf(arr), reflect.ValueOf([]string{})) },
		func() { reflect.Copy(reflect.ValueOf(dst), reflect.ValueOf("str")) },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// 3 [1 2 3]
// 1 [9 2 3]
// 2 [a b  ]
// 3 [7 8 9]
// 4 hell
// [1 1 2 3 4]
// [3 4 5 4 5]
// [[100] [2]]
// recovered: reflect: call of reflect.Copy on int Value
// recovered: reflect: call of reflect.Copy on int Value
// recovered: reflect.Copy: int != string
// recovered: reflect: unknown method using unaddressable value
// recovered: reflect: call of reflect.Copy on string Value
