// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type S struct{ A int }

func main() {
	it := reflect.TypeOf(0)
	fmt.Println(reflect.PointerTo(it), reflect.PtrTo(it), reflect.PointerTo(it) == reflect.TypeOf(new(int)))
	fmt.Println(reflect.SliceOf(it), reflect.SliceOf(it) == reflect.TypeOf([]int{}))
	st := reflect.TypeOf(S{})
	fmt.Println(reflect.PointerTo(st), reflect.SliceOf(st), reflect.PointerTo(reflect.SliceOf(st)))
	pp := reflect.PointerTo(reflect.PointerTo(it))
	fmt.Println(pp, pp.Kind(), pp.Elem().Elem())
	ss := reflect.SliceOf(reflect.SliceOf(reflect.TypeOf("")))
	fmt.Println(ss, ss.Kind(), ss.Elem().Elem())
	// a type the program never wrote
	ft := reflect.TypeOf(float32(0))
	n := reflect.New(ft)
	n.Elem().SetFloat(2.5)
	fmt.Println(n.Type(), n.Elem().Float(), *(n.Interface().(*float32)))
	sl := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(uint16(0))), 3, 3)
	sl.Index(1).SetUint(65535)
	fmt.Println(sl.Type(), sl.Interface())
	fmt.Println(reflect.PointerTo(st).Elem() == st)
}

// Output:
// *int *int true
// []int true
// *main.S []main.S *[]main.S
// **int ptr int
// [][]string slice string
// *float32 2.5 2.5
// []uint16 [0 65535 0]
// true
