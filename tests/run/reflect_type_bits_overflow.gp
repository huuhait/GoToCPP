// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	i8, i16, i32, i64 := reflect.TypeOf(int8(0)), reflect.TypeOf(int16(0)), reflect.TypeOf(int32(0)), reflect.TypeOf(int64(0))
	for _, t := range []reflect.Type{i8, i16, i32, i64} {
		fmt.Println(t, t.Bits(), t.OverflowInt(127), t.OverflowInt(128), t.OverflowInt(-129), t.OverflowInt(1<<31), t.OverflowInt(1<<40))
	}
	u8, u16, u32, u64 := reflect.TypeOf(uint8(0)), reflect.TypeOf(uint16(0)), reflect.TypeOf(uint32(0)), reflect.TypeOf(uint64(0))
	for _, t := range []reflect.Type{u8, u16, u32, u64} {
		fmt.Println(t, t.Bits(), t.OverflowUint(255), t.OverflowUint(256), t.OverflowUint(1<<32), t.OverflowUint(1<<63))
	}
	f32, f64 := reflect.TypeOf(float32(0)), reflect.TypeOf(0.0)
	fmt.Println(f32.Bits(), f64.Bits(), f32.OverflowFloat(1e38), f32.OverflowFloat(1e39), f32.OverflowFloat(-1e39), f64.OverflowFloat(1e300))
	c64, c128 := reflect.TypeOf(complex64(0)), reflect.TypeOf(complex128(0))
	fmt.Println(c64.Bits(), c128.Bits(), c64.OverflowComplex(complex(1e39, 0)), c64.OverflowComplex(complex(0, 1e30)), c128.OverflowComplex(complex(1e300, 1e300)))
}

// Output:
// int8 8 false true true true true
// int16 16 false false false true true
// int32 32 false false false true true
// int64 64 false false false false false
// uint8 8 false true true true
// uint16 16 false false true true
// uint32 32 false false true true
// uint64 64 false false false false
// 32 64 false true true false
// 64 128 true false false
