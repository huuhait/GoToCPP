// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	s := []string{"a", "b", "c"}
	a := [3]int{10, 20, 30}
	str := "héy"
	sv, av, tv := reflect.ValueOf(s), reflect.ValueOf(a), reflect.ValueOf(str)
	for i := 0; i < sv.Len(); i++ {
		fmt.Print(sv.Index(i), " ", sv.Index(i).CanSet(), sv.Index(i).CanAddr(), ";")
	}
	fmt.Println()
	for i := 0; i < av.Len(); i++ {
		fmt.Print(av.Index(i), " ", av.Index(i).CanSet(), av.Index(i).CanAddr(), ";")
	}
	fmt.Println()
	for i := 0; i < tv.Len(); i++ {
		e := tv.Index(i)
		fmt.Print(e.Kind(), e.Uint(), " ", e.CanSet(), ";")
	}
	fmt.Println()
	pa := reflect.ValueOf(&a).Elem()
	fmt.Println(pa.Index(1).CanSet(), pa.Index(1).CanAddr())
	pa.Index(1).SetInt(99)
	fmt.Println(a)
	for _, f := range []func(){
		func() { sv.Index(3) }, func() { sv.Index(-1) }, func() { av.Index(3) }, func() { tv.Index(10) }, func() { reflect.ValueOf(1).Index(0) },
		func() { reflect.ValueOf(map[int]int{}).Index(0) }, func() { sv.Index(0).SetString("x") },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// a true true;b true true;c true true;
// 10 false false;20 false false;30 false false;
// uint8 104 false;uint8 195 false;uint8 169 false;uint8 121 false;
// true true
// [10 99 30]
// recovered: reflect: slice index out of range
// recovered: reflect: slice index out of range
// recovered: reflect: array index out of range
// recovered: reflect: string index out of range
// recovered: reflect: call of reflect.Value.Index on int Value
// recovered: reflect: call of reflect.Value.Index on map Value
// recovered: <nil>
