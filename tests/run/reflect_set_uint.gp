// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type MyU uint16

func main() {
	var u uint
	var u8 uint8
	var u16 uint16
	var u32 uint32
	var u64 uint64
	var up uintptr
	var mu MyU
	set := func(p any, x uint64) {
		v := reflect.ValueOf(p).Elem()
		v.SetUint(x)
		fmt.Printf("%v %v overflow=%v\n", v.Type(), v.Uint(), v.OverflowUint(x))
	}
	set(&u, 5)
	set(&u8, 255)
	set(&u8, 256)
	set(&u16, 70000)
	set(&u32, 1<<32)
	set(&u64, 1<<63)
	set(&up, 12345)
	set(&mu, 65535)
	fmt.Println(u, u8, u16, u32, u64, up, mu)
}

// Output:
// uint 5 overflow=false
// uint8 255 overflow=false
// uint8 0 overflow=true
// uint16 4464 overflow=true
// uint32 0 overflow=true
// uint64 9223372036854775808 overflow=false
// uintptr 12345 overflow=false
// main.MyU 65535 overflow=false
// 5 0 4464 0 9223372036854775808 12345 65535
