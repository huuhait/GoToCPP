// All comparisons, on every basic type: the same C++ operators (gp::string
// compares its bytes as unsigned bytes, like Go). A comparison inside a
// comparison gets parentheses.
package main

import "fmt"

func ints(a, b int64) {
	fmt.Println(a == b, a != b, a < b, a <= b, a > b, a >= b)
}

func uints(a, b uint64) {
	fmt.Println(a == b, a != b, a < b, a <= b, a > b, a >= b)
}

func small(a, b int8, c, d uint16) {
	fmt.Println(a == b, a != b, a < b, a <= b, a > b, a >= b, c < d, c >= d, a < -1, c > 65534)
}

func floats(a, b float64) {
	fmt.Println(a == b, a != b, a < b, a <= b, a > b, a >= b)
}

func strs(a, b string) {
	fmt.Println(a == b, a != b, a < b, a <= b, a > b, a >= b)
}

func bools(a, b bool) {
	fmt.Println(a == b, a != b, a && b, a || b, !a, !a == b, a == (b == a), a != b == !b)
}

func main() {
	ints(1, 2)
	ints(-9223372036854775808, 9223372036854775807)
	ints(5, 5)
	uints(18446744073709551615, 0)
	uints(1<<63, 1<<63-1)
	small(-128, 127, 65535, 0)
	small(-1, -1, 7, 7)
	zero := 0.0
	floats(1.5, 2.5)
	floats(zero, -zero)
	floats(zero/zero, zero/zero)
	floats(zero/zero, 1)
	floats(-1/zero, 1/zero)
	strs("abc", "abd")
	strs("abc", "ab")
	strs("", "")
	strs("é", "z")
	strs("\xff", "\x7f")
	strs("a\x00b", "a\x00c")
	bools(true, false)
	bools(false, false)
	var u32 uint32 = 4000000000
	var i32 int32 = -2147483648
	var f32 float32 = 0.1
	fmt.Println(u32 > 3999999999, i32 < -2147483647, f32 == 0.1, float64(f32) == 0.1, u32 == 4000000000)
	x, y := 3, 4
	fmt.Println(x < y == true, (x < y) != (y < x), x+1 < y*2, x&1 == 1, x|y == 7, x^y != 0)
}
