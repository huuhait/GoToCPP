// phase: 02

// The small integer types wrap like Go: arithmetic on int8, int16, uint8
// and uint16 stays in the type, also inside a bigger expression and a
// comparison.
package main

import "fmt"

func main() {
	var a, b uint8 = 200, 100
	fmt.Println(a+b, a+b > 250, a*2, a-b-b-b, -a, ^a)
	var c int8 = 100
	fmt.Println(c+c, c*3, -c-c, c<<1, c>>1)
	var x int16 = 30000
	fmt.Println(x+x, x*2/2, x*4, -x-x-x)
	var y uint16 = 65000
	fmt.Println(y+1000, y*2, y<<4, uint16(1)-y)
	var z int32 = 2147483647
	fmt.Println(z+1, z*2, -z-2)

	var acc uint8
	for i := range 20 {
		acc += uint8(i * 17)
	}
	fmt.Println("acc", acc)

	var neg int8 = -128
	neg--
	fmt.Println(neg)
	neg = -128
	fmt.Println(-neg, neg*-1, neg/-1, neg%-1)

	var d uint8 = '7'
	fmt.Println(d-'0', d-'0' <= 9, d-'a' <= 9, d-'a')

	h := uint16(0xFFFF)
	h++
	fmt.Println(h, h == 0)

	var mix int16 = 300
	fmt.Println(int8(mix), uint8(mix), int8(mix)*2, uint8(mix)+uint8(mix))
}

// Output:
// 44 false 144 156 56 55
// -56 44 56 -56 50
// -5536 -2768 -11072 -24464
// 464 64464 56960 537
// -2147483648 -2 2147483647
// acc 158
// 127
// -128 -128 -128 0
// 7 true false 214
// 0 true
// 44 44 88 88
