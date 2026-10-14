// phase: 02
// Stderr: (?s)^true false\n-128 255 -32768 65535\n-2147483648 4294967295\n-9223372036854775808 18446744073709551615 42\n\+1\.500000e\+000 -2\.500000e-001 \+0\.000000e\+000\n\+1\.000000e\+010 \+1\.100000e\+000 true\nhello  world\n97 48\n\nab1truex\n$

// The built-ins println and print write to stderr in the runtime's own
// format: floats as +d.dddddde+ddd, values separated by a space (println
// only), no quotes around strings.
package main

import "fmt"

func main() {
	fmt.Println("only this line goes to stdout")
	println(true, false)
	var i8 int8 = -128
	var u8 uint8 = 255
	var i16 int16 = -32768
	var u16 uint16 = 65535
	println(i8, u8, i16, u16)
	var i32 int32 = -2147483648
	var u32 uint32 = 4294967295
	println(i32, u32)
	var i64 int64 = -9223372036854775808
	var u64 uint64 = 18446744073709551615
	var n int = 42
	println(i64, u64, n)
	var z float64
	println(1.5, -0.25, z)
	var f32 float32 = 1.1
	println(1e10, 1.1, f32 > 1)
	println("hello", "", "world")
	println('a', '0')
	println()
	print("a", "b", 1, true, "x\n")
}

// Output:
// only this line goes to stdout
