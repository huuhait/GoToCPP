// phase: 04

// C unions (all members share memory), bit-fields (read and write), anonymous union and struct
// members, a function pointer field called through C, enum values, constant macros of every kind,
// function-like macros called like functions, and arrays in structs.
package main

import (
	"fmt"

	"<iterator>"

	"./cunion.h"
)

func main() {
	var n cunion.Num
	n.f = 1.0
	fmt.Println(uint32(n.i), int(n.bytes[3]), int(n.bytes[2]))
	n.i = 0
	n.bytes[0] = 255
	fmt.Println(int32(n.i), int(std.size(n.bytes)))
	bits := cunion.cu_float_bits(-2.0)
	fmt.Println(uint32(bits.i) == 0xc0000000, uint32(bits.i)>>24)

	// Bit-fields: read through int(...) conversions of a C field.
	f := cunion.cu_flags(5)
	fmt.Println(uint(f.ready), uint(f.mode), uint(f.level), int(f.signed_part))
	f.level = 9 // 4 bits
	f.signed_part = -3
	f.mode++
	fmt.Println(uint(f.level), int(f.signed_part), uint(f.mode))

	it := cunion.cu_item(3, 2.5)
	fmt.Println(int(it.kind), float64(it.dval), int(it.pos.x), int(it.pos.y), C.GoString(&it.name[0]))
	it.ival = 7 // the anonymous union member is reached directly
	fmt.Println(int(it.ival), int(cunion.cu_apply(&it, 6, 7)))
	even := cunion.cu_item(4, 0)
	fmt.Println(int(cunion.cu_apply(&even, 6, 7)))
	it.op = cunion.cu_mul // a C function in a function pointer field
	fmt.Println(int(cunion.cu_apply(&it, 6, 7)))

	// Enums and macros.
	var d cunion.enum_Weekday = cunion.WED
	fmt.Println(int(cunion.MON), int(cunion.TUE), int(d), int(cunion.SUN), int(cunion.ON))
	fmt.Println(cunion.CU_VERSION, C.GoString(cunion.CU_NAME), cunion.CU_PI)
	fmt.Println(uint(cunion.CU_FLAG(4)), int(cunion.CU_SQUARE(7)), int(cunion.CU_MAX(3, 9)))
	k := 5
	fmt.Println(int(cunion.CU_SQUARE(k+1)), int(cunion.CU_MAX(C.int(k), 2)))
}

// Output:
// 1065353216 63 128
// 255 4
// true 192
// 1 5 0 0
// 9 -3 6
// 3 2.5 30 300 item
// 7 13
// 42
// 42
// 1 2 3 7 10
// 3 cunion 3.5
// 16 49 9
// 36 5
