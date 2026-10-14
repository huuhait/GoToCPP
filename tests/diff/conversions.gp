// phase: 02

// Conversions between the integer types (truncation and sign extension)
// and from integers to floats, on a grid of values.
package main

import "fmt"

func value(i int) int64 {
	switch i {
	case 0:
		return 0
	case 1:
		return 1
	case 2:
		return -1
	case 3:
		return 127
	case 4:
		return 128
	case 5:
		return 255
	case 6:
		return 256
	case 7:
		return -129
	case 8:
		return 32768
	case 9:
		return 65535
	case 10:
		return -32769
	case 11:
		return 2147483648
	case 12:
		return -2147483649
	case 13:
		return 4294967296
	case 14:
		return 9007199254740993
	case 15:
		return -9223372036854775808
	}
	return 9223372036854775807
}

func main() {
	for i := range 17 {
		v := value(i)
		fmt.Println(v, int8(v), uint8(v), int16(v), uint16(v), int32(v), uint32(v), uint64(v), uint(v), uintptr(v))
		fmt.Println("  ", float64(v), float32(v), float64(uint64(v)), float32(uint64(v)), float64(int32(v)), float64(uint8(v)))
		fmt.Println("  ", int64(int8(v)), int64(uint8(v)), int(int16(v)), uint64(int32(v)), int8(uint8(v)), uint8(int8(v)))
	}
	var u uint64 = 18446744073709551615
	fmt.Println(float64(u), float32(u), float64(u>>11), float64(u-1024), float64(u-2048))
	var r rune = 0x10FFFF
	fmt.Println(r, byte(r), uint16(r), int8(r))
}
