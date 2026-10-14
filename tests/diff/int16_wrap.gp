// phase: 02

// int16 arithmetic wraps like Go: every binary operator, unary - and ^, the
// op-assign forms and ++/-- on a grid of values, also inside bigger
// expressions and comparisons (C++ promotes int16 to int first).
package main

import "fmt"

func value(i int) int16 {
	switch i {
	case 0:
		return -32768
	case 1:
		return -32767
	case 2:
		return -1000
	case 3:
		return -256
	case 4:
		return -1
	case 5:
		return 0
	case 6:
		return 1
	case 7:
		return 2
	case 8:
		return 255
	case 9:
		return 1000
	case 10:
		return 32767
	}
	return 0
}

func main() {
	const n = 11
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
	var x int16 = value(n - 1)
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
	var y int16
	y--
	fmt.Println(y, y+1, y*y, y/2)
}
