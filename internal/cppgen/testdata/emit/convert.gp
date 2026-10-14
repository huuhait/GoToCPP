// Conversions between every pair of basic number types (T(x) in C++, or
// gp::f2i<T> from a float to an integer type), at the edge values of each
// type: integers wrap or sign-extend like Go, and integers and floats round to
// the nearest value. (Out-of-range float-to-integer conversions are in f2i.gp:
// their result depends on the CPU.)
package main

import "fmt"

func fromInt(x int) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromInt8(x int8) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromInt16(x int16) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromInt32(x int32) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromInt64(x int64) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromUint(x uint) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromUint8(x uint8) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromUint16(x uint16) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromUint32(x uint32) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromUint64(x uint64) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromUintptr(x uintptr) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x), float32(x), float64(x))
}

func fromFloat32(x float32) {
	fmt.Println(float32(x), float64(x))
}

// intsFloat32 gets only values that every integer type holds after the truncation.
func intsFloat32(x float32) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x))
}

func fromFloat64(x float64) {
	fmt.Println(float32(x), float64(x))
}

// intsFloat64 gets only values that every integer type holds after the truncation.
func intsFloat64(x float64) {
	fmt.Println(int(x), int8(x), int16(x), int32(x), int64(x), uint(x), uint8(x), uint16(x), uint32(x), uint64(x), uintptr(x))
}

func main() {
	fromInt(-9223372036854775808)
	fromInt(-9223372036854775807)
	fromInt(-4294967297)
	fromInt(-65536)
	fromInt(-129)
	fromInt(-1)
	fromInt(0)
	fromInt(1)
	fromInt(255)
	fromInt(65535)
	fromInt(2147483648)
	fromInt(9007199254740993)
	fromInt(9223372036854775807)
	fromInt8(-128)
	fromInt8(-1)
	fromInt8(0)
	fromInt8(1)
	fromInt8(127)
	fromInt16(-32768)
	fromInt16(-129)
	fromInt16(-1)
	fromInt16(0)
	fromInt16(128)
	fromInt16(255)
	fromInt16(32767)
	fromInt32(-2147483648)
	fromInt32(-32769)
	fromInt32(-1)
	fromInt32(0)
	fromInt32(65535)
	fromInt32(16777217)
	fromInt32(2147483647)
	fromInt64(-9223372036854775808)
	fromInt64(-1)
	fromInt64(0)
	fromInt64(4294967295)
	fromInt64(9007199254740993)
	fromInt64(9223372036854775807)
	fromUint(0)
	fromUint(1)
	fromUint(255)
	fromUint(4294967296)
	fromUint(18446744073709551615)
	fromUint8(0)
	fromUint8(1)
	fromUint8(127)
	fromUint8(128)
	fromUint8(255)
	fromUint16(0)
	fromUint16(255)
	fromUint16(32768)
	fromUint16(65535)
	fromUint32(0)
	fromUint32(2147483648)
	fromUint32(16777217)
	fromUint32(4294967295)
	fromUint64(0)
	fromUint64(9007199254740993)
	fromUint64(9223372036854775808)
	fromUint64(9223372036854775809)
	fromUint64(18446744073709550591)
	fromUint64(18446744073709551615)
	fromUintptr(0)
	fromUintptr(4294967296)
	fromUintptr(18446744073709551615)
	fromFloat32(0)
	fromFloat32(-0.5)
	fromFloat32(100.75)
	fromFloat32(16777217)
	fromFloat32(3.4028235e38)
	fromFloat32(1e-45)
	fromFloat64(0)
	fromFloat64(100.75)
	fromFloat64(127.9)
	fromFloat64(1e-320)
	fromFloat64(3.4028235677973366e38)
	fromFloat64(3.4028235677973362e38)
	fromFloat64(1e300)
	fromFloat64(16777217)
	fromFloat64(16777216.5)
	intsFloat32(0)
	intsFloat32(-0.5)
	intsFloat32(100.75)
	intsFloat32(127.9)
	intsFloat64(-0.99)
	intsFloat64(1.5)
	intsFloat64(126.999)
	// The aliases byte and rune, and a parenthesized type.
	var r rune = -1
	var b byte = 200
	fmt.Println(byte(r), rune(byte(r)), (int8)(r), (uint16)(r), rune(b), byte(b+100), int(b)+100)
}
