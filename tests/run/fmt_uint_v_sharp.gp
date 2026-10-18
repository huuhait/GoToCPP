// phase: 05

// %#v of unsigned integers is hex (0x41), of signed ones decimal: the sharpV flag; named types
// print the same.
package main

import "fmt"

type Count uint16

type Level int8

func main() {
	fmt.Printf("%#v %#v %#v %#v %#v\n", uint(1), uint8(255), uint64(1<<40), uintptr(7), uint16(3))
	fmt.Printf("%#v %#v %#v %#v\n", 1, int8(-3), int64(5), int32(9))
	fmt.Printf("%#v %v %d %#v\n", Count(9), Count(9), Count(9), Level(-4))
	fmt.Printf("%#v %#v\n", []uint8{1, 2}, [3]uint16{1, 2, 3})
	fmt.Printf("%#v\n", map[uint8]uint32{1: 2})
}

// Output:
// 0x1 0xff 0x10000000000 0x7 0x3
// 1 -3 5 9
// 0x9 9 9 -4
// []byte{0x1, 0x2} [3]uint16{0x1, 0x2, 0x3}
// map[uint8]uint32{0x1:0x2}
