// phase: 02

// uint8 arithmetic wraps like Go: every binary operator, unary - and ^, the
// op-assign forms and ++/-- on a grid of values, also inside bigger
// expressions and comparisons (C++ promotes uint8 to int first).
package main

import "fmt"

func value(i int) uint8 {
	switch i {
	case 0:
		return 0
	case 1:
		return 1
	case 2:
		return 2
	case 3:
		return 7
	case 4:
		return 15
	case 5:
		return 16
	case 6:
		return 100
	case 7:
		return 127
	case 8:
		return 128
	case 9:
		return 200
	case 10:
		return 254
	case 11:
		return 255
	}
	return 0
}

func main() {
	const n = 12
	for i := range n {
		a := value(i)
		fmt.Println("a", a, -a, ^a, a+a, a*a, a<<1, a>>1, a<<7, a>>7)
		for j := range n {
			b := value(j)
			fmt.Println(a, b, a+b, a-b, a*b, a&b, a|b, a^b, a&^b, a+b > a, a*b < b, a-b+b == a, (a+b)/2, a*3-b*5)
			if b != 0 {
				fmt.Println("  div", a/b, a%b, (a+b)/b, (a*b)%b)
			}
		}
	}
	var x uint8 = value(n - 1)
	x++
	fmt.Println(x)
	x--
	x--
	fmt.Println(x)
	x += value(n - 2)
	x *= 3
	x -= value(1)
	x <<= 3
	x >>= 1
	x |= 1
	x &= value(n - 1)
	x ^= value(2)
	x &^= value(3)
	fmt.Println(x)
	var y uint8
	y--
	fmt.Println(y, y+1, y*y, y/2)
}
