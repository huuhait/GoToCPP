// phase: 02
// diff: stderr

// The built-ins println and print use the runtime's number format, not
// fmt's: floats as +d.dddddde+ddd, NaN and +Inf, a space between println's
// operands and none between print's. Every basic type.
package main

func main() {
	var b bool
	var i8 int8 = -128
	var i16 int16 = -32768
	var i32 int32 = -2147483648
	var i64 int64 = -9223372036854775808
	var i int = 9223372036854775807
	var u8 uint8 = 255
	var u16 uint16 = 65535
	var u32 uint32 = 4294967295
	var u64 uint64 = 18446744073709551615
	var u uint = 1
	var up uintptr = 0xdeadbeef
	var r rune = 'é'
	var by byte = 'A'
	println(b, !b, i8, i16, i32, i64, i)
	println(u8, u16, u32, u64, u, up, r, by)
	print(b, i8, u8, "|", 1, 2, "\n")
	zero := 0.0
	var f32 float32 = 0.1
	println(0.0, -zero, 1.0, -1.0, 0.5, 1e-7, 123456789.0, 1e100, 1e-100)
	println(1.7976931348623157e308, 4.9e-324, 1/zero, -1/zero, zero/zero)
	println(f32, float32(1e38), float32(1.4e-45), 0.1+0.2, 2.0/3.0)
	println(9.999999e9, 9.9999995e9, 0.30000000000000004, 1e21, 1e20)
	println("string", "", "two words", "tab\there", "é")
	print("print ", "joins ", "strings", 1.5, true, "\n")
	print()
	println()
	println("end")
}
