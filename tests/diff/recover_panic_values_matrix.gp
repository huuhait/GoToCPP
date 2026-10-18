// phase: 05

package main

import "fmt"

func catch(f func()) (v any) {
	defer func() { v = recover() }()
	f()
	return
}

func main() {
	var i8 int8 = -8
	var u8 uint8 = 200
	var i16 int16 = -1600
	var u16 uint16 = 65000
	var i32 int32 = -32
	var u32 uint32 = 4000000000
	var i64 int64 = -64
	var u64 uint64 = 1 << 63
	var up uintptr = 4096
	var f32 float32 = 0.5
	var c64 complex64 = 1 + 2i
	vals := []func(){
		func() { panic(i8) }, func() { panic(u8) }, func() { panic(i16) }, func() { panic(u16) },
		func() { panic(i32) }, func() { panic(u32) }, func() { panic(i64) }, func() { panic(u64) },
		func() { panic(up) }, func() { panic(f32) }, func() { panic(c64) }, func() { panic(1 + 2i) },
		func() { panic('r') }, func() { panic("str") }, func() { panic(1.25) }, func() { panic(false) },
		func() { panic(-7) },
	}
	for _, f := range vals {
		fmt.Println(catch(f))
	}
	fmt.Printf("%v %d %s|\n", catch(func() { panic(12) }), catch(func() { panic(34) }), catch(func() { panic("s") }))
}
