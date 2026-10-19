// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	for k := reflect.Invalid; k <= reflect.UnsafePointer; k++ {
		fmt.Printf("%d %s %v\n", k, k, k)
	}
	fmt.Println(reflect.Kind(27), reflect.Kind(100))
	fmt.Println(reflect.Ptr == reflect.Pointer, reflect.Ptr)
	fmt.Println(reflect.RecvDir, reflect.SendDir, reflect.BothDir, reflect.ChanDir(0), reflect.ChanDir(9))
}

// Output:
// 0 invalid invalid
// 1 bool bool
// 2 int int
// 3 int8 int8
// 4 int16 int16
// 5 int32 int32
// 6 int64 int64
// 7 uint uint
// 8 uint8 uint8
// 9 uint16 uint16
// 10 uint32 uint32
// 11 uint64 uint64
// 12 uintptr uintptr
// 13 float32 float32
// 14 float64 float64
// 15 complex64 complex64
// 16 complex128 complex128
// 17 array array
// 18 chan chan
// 19 func func
// 20 interface interface
// 21 map map
// 22 ptr ptr
// 23 slice slice
// 24 string string
// 25 struct struct
// 26 unsafe.Pointer unsafe.Pointer
// kind27 kind100
// true ptr
// <-chan chan<- chan ChanDir0 ChanDir9
