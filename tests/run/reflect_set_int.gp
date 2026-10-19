// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type MyInt int32

func main() {
	var i int
	var i8 int8
	var i16 int16
	var i32 int32
	var i64 int64
	var mi MyInt
	set := func(p any, x int64) {
		v := reflect.ValueOf(p).Elem()
		v.SetInt(x)
		fmt.Printf("%v %v overflow=%v\n", v.Type(), v.Int(), v.OverflowInt(x))
	}
	set(&i, -5)
	set(&i8, 100)
	set(&i8, 200)
	set(&i8, -129)
	set(&i16, 40000)
	set(&i32, 1<<31)
	set(&i64, -1<<63)
	set(&mi, 7)
	fmt.Println(i, i8, i16, i32, i64, mi)
	arr := []int8{1, 2, 3}
	reflect.ValueOf(arr).Index(1).SetInt(-100)
	fmt.Println(arr)
}

// Output:
// int -5 overflow=false
// int8 100 overflow=false
// int8 -56 overflow=true
// int8 127 overflow=true
// int16 -25536 overflow=true
// int32 -2147483648 overflow=true
// int64 -9223372036854775808 overflow=false
// main.MyInt 7 overflow=false
// -5 127 -25536 -2147483648 -9223372036854775808 7
// [1 -100 3]
