// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	var a [3]int
	v := reflect.ValueOf(&a).Elem()
	for i := 0; i < v.Len(); i++ {
		v.Index(i).SetInt(int64(i * 10))
	}
	fmt.Println(a, v.CanSet(), v.Index(0).CanSet())
	var grid [2][3]string
	g := reflect.ValueOf(&grid).Elem()
	g.Index(1).Index(2).SetString("x")
	g.Index(0).Set(reflect.ValueOf([3]string{"a", "b", "c"}))
	fmt.Println(grid)
	b := [3]int{1, 2, 3}
	bv := reflect.ValueOf(b)
	fmt.Println(bv.Index(0).CanSet(), bv.CanAddr())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		bv.Index(0).SetInt(5)
	}()
	var arrs [2]struct{ N int }
	reflect.ValueOf(&arrs).Elem().Index(1).Field(0).SetInt(4)
	fmt.Println(arrs)
	reflect.ValueOf(&a).Elem().Set(reflect.ValueOf([3]int{7, 8, 9}))
	fmt.Println(a)
}

// Output:
// [0 10 20] true true
// [[a b c] [  x]]
// false false
// recovered: reflect: reflect.Value.SetInt using unaddressable value
// [{0} {4}]
// [7 8 9]
