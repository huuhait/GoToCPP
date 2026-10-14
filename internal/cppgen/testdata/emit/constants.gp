// Constants are folded by go/types (exact, arbitrary precision) and written as
// C++ literals of their final type, also in untyped contexts.
package main

import "fmt"

const (
	big   = 1 << 100
	huge  = big * big
	third = 1.0 / 3
	tiny  = 1e-300 * 1e-300 * 1e300
	kb    = 1 << (10 * (iota - 3))
	mb
	gb
)

func main() {
	fmt.Println(big>>98, huge>>199, big/(1<<90), big%7, -big>>99)
	fmt.Println(third, float32(third), third*3 == 1, tiny)
	fmt.Println(kb, mb, gb, gb*1024*1024*1024)
	fmt.Println(9223372036854775807, -9223372036854775808, uint64(18446744073709551615), 1<<62, -1<<63)
	fmt.Println(2147483647, -2147483648, 2147483648, -2147483649, uint32(4294967295), int32(-2147483648))
	fmt.Println(0.1+0.2, 1e308*10/100, float32(1e38)*3/1e10, float32(16777217), float64(float32(0.1)))
	fmt.Println('a', 'é', '\n', "a"+"b", len("héllo"), "x" < "y", 7/2, -7/2, 7%-3, 7.0/2)
	var f32 float32 = 1 << 24
	var u8 uint8 = 255
	var i16 int16 = -32768
	var u64 uint64 = 1 << 63
	x := 3
	fmt.Println(f32+1, u8+1, i16-1, u64*2, x*1e2, float64(x)*1.5, x/2*2.0)
	println(big>>90, third, 'x', uint8(200), 1e100, float32(0.1), true && !false)
	const c = 10
	var d int8 = c * 12
	var e float32 = c / 4
	var f float64 = c / 4.0
	fmt.Println(d, e, f, c/3, c/3.0)
}
