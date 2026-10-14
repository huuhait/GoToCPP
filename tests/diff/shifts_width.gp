// phase: 02

// Shifts by a variable count, including counts at and past the width of
// the type: Go gives 0 for << and unsigned >>, and 0 or -1 for signed >>
// (C++ makes these undefined). Counts of every unsigned and signed type.
package main

import "fmt"

func main() {
	counts := 0
	for n := uint(0); n <= 70; n++ {
		if n > 10 && n < 60 && n%8 != 0 && n%8 != 7 && n%8 != 1 {
			continue
		}
		counts++
		var i8 int8 = -100
		var u8 uint8 = 0xA5
		var i16 int16 = -12345
		var u16 uint16 = 0xBEEF
		var i32 int32 = -123456789
		var u32 uint32 = 0xDEADBEEF
		var i64 int64 = -1234567890123
		var u64 uint64 = 0xFEEDFACECAFEBEEF
		fmt.Println(n, i8<<n, i8>>n, u8<<n, u8>>n, i16<<n, i16>>n, u16<<n, u16>>n)
		fmt.Println(n, i32<<n, i32>>n, u32<<n, u32>>n, i64<<n, i64>>n, u64<<n, u64>>n)
		pos := int64(1234567)
		fmt.Println(n, pos<<n, pos>>n, -pos>>n, 1<<n, -1>>n)
	}
	fmt.Println("counts", counts)

	x := 0xFF
	var c8 uint8 = 200
	var c16 int16 = 64
	var c32 int32 = 63
	var c64 uint64 = 1 << 40
	fmt.Println(x<<c8, x>>c8, x<<c16, x>>c16, x<<c32, x>>c32, x<<c64, x>>c64)
	m := -0xFF
	fmt.Println(m<<c8, m>>c8, m>>c16, m>>c32, m>>c64)
	var big uint64 = 18446744073709551615
	fmt.Println(1<<(big>>58), x>>big, x<<big)
	var s8 int8 = 1
	for range 9 {
		s8 <<= 1
		fmt.Print(s8, " ")
	}
	fmt.Println()
	var t uint32 = 1
	k := 31
	fmt.Println(t<<k, t<<(k+1), t<<k>>k, (t<<k)>>(k+1))
}
