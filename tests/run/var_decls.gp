// phase: 02

// Declarations and assignments: var with zero values of every basic type,
// var with types and values, := , parallel assignment (swap, rotate),
// op-assign for every operator, ++ and --, and the blank identifier.
package main

import "fmt"

func pair() (int, string) {
	return 7, "seven"
}

func main() {
	var (
		b   bool
		i   int
		i8  int8
		u16 uint16
		f32 float32
		f64 float64
		s   string
		r   rune
		by  byte
	)
	fmt.Println(b, i, i8, u16, f32, f64, s == "", r, by)

	var x, y int = 3, 4
	var p, q = "p", 2.5
	fmt.Println(x, y, p, q)

	x, y = y, x
	fmt.Println(x, y)
	a, c, d := 1, 2, 3
	a, c, d = c, d, a
	fmt.Println(a, c, d)

	n := 100
	n += 5
	n -= 3
	n *= 2
	n /= 4
	n %= 13
	fmt.Println(n)
	m := 0xF0
	m |= 0x0F
	m &= 0x3C
	m ^= 0xFF
	m <<= 2
	m >>= 1
	m &^= 0x10
	fmt.Println(m)
	n++
	n++
	n--
	fmt.Println(n)

	_, name := pair()
	num, _ := pair()
	fmt.Println(name, num)
	var _ = 5
	_ = name

	num, extra := pair()
	fmt.Println(num, extra)

	var g float64 = 1
	g /= 3
	g *= 3
	fmt.Println(g)
	str := "a"
	str += "b"
	str += str
	fmt.Println(str)
}

// Output:
// false 0 0 0 0 0 true 0 0
// 3 4 p 2.5
// 4 3
// 2 3 1
// 12
// 390
// 13
// seven 7
// 7 seven
// 1
// abab
