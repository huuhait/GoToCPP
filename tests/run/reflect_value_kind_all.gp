// phase: 09

package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

type S struct{ A int }
type I interface{ M() }

func main() {
	var i I
	var up unsafe.Pointer
	x := 5
	all := []any{true, 1, int8(1), int16(1), int32(1), int64(1), uint(1), uint8(1), uint16(1), uint32(1), uint64(1), uintptr(1), float32(1), 1.0,
		complex64(1), complex128(1), [1]int{}, make(chan int), func() {}, map[int]int{}, &x, []int{}, "s", S{}, up}
	seen := map[reflect.Kind]bool{}
	for _, v := range all {
		k := reflect.ValueOf(v).Kind()
		seen[k] = true
		fmt.Print(k, ";")
	}
	fmt.Println()
	// an interface Kind comes only from a field or element
	v := reflect.ValueOf(&struct{ F I }{i}).Elem().Field(0)
	fmt.Println(v.Kind(), len(seen))
	for k := reflect.Bool; k <= reflect.UnsafePointer; k++ {
		if !seen[k] && k != reflect.Interface {
			fmt.Println("missing", k)
		}
	}
}

// Output:
// bool;int;int8;int16;int32;int64;uint;uint8;uint16;uint32;uint64;uintptr;float32;float64;complex64;complex128;array;chan;func;map;ptr;slice;string;struct;unsafe.Pointer;
// interface 25
