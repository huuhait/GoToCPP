// Go's numeric types and their conversions: ints of every width, unsigned wraps, uintptr,
// floats and float32 rounding, byte/rune/string conversions, min/max, shifts.
package main

type celsius float64

func main() {
	var i int = -3
	var u uint = 7
	var u8 uint8 = 255
	var i64 int64 = 1 << 40
	var f float64 = 2.5
	var f32 float32 = 1.5
	var r rune = 'A'
	var by byte = 0x41
	println(i, u, u8, i64)
	println(int(i), uint(i), int64(u), uint8(u))
	println(f, f32, float64(f32), int(f))
	println(r, byte(r), string(r), by)
	println(min(3, 5), max(3, 5), min(2.5, 1.5))
	println(int8(-1), uint16(65535), int32(-2))
	var c celsius = 21.5
	println(float64(c), c+1)
	var p uintptr = 16
	println(uintptr(8), p)
	println(1<<62, -1<<63)
	println(255&15, 255|256, 255^15, ^0 == -1)
}
