// phase: 12
// opaque: reject

// Gap E1: a C/C++ constant where Go needs a constant. The names are probed first and put into the
// program as real constants before the Go type check, so an array length, a const declaration, a
// case and a shift count see numbers, strings and characters, not opaque names. The C++ twin
// computes the same with the macros themselves.
package main

import (
	"fmt"

	"<climits>"
	"<cmath>"
	"<cstdio>"
	"./consts.hpp"
)

const bufHalf = cstdio.BUFSIZ / 2
const tau = cmath.M_PI * 2
const big = consts.BIG
const low = uint32(consts.MASK) & 0xFF
const tenths = consts.TENTH * 3
const greeting = consts.GREETING + ", world"
const sep = consts.SEP
const esc = consts.ESC
const limit = consts.LIMIT
const rows = consts.kRows * consts.ROWS
const shape = consts.geo.kSides + consts.Dim.kWidth + consts.kDim.h
const enumSum = consts.GREEN + consts.BLUE
const minInt = climits.INT_MIN

var buf [cstdio.BUFSIZ]byte
var table [consts.ROWS][consts.COLS]int
var frame [consts.FRAME / 2]uint8
var word [len(consts.GREETING)]byte

type Frame struct {
	cells [consts.ALIAS]int
	tail  [rows]byte
}

func classify(c int) string {
	switch c {
	case cstdio.EOF:
		return "eof"
	case consts.NEG, consts.ZERO:
		return "neg-or-zero"
	case consts.RED + 1, consts.GREEN:
		return "enum"
	case consts.COLS * 10:
		return "forty"
	}
	return "other"
}

// A byte cannot be -1: Go's constant rules would reject the case, C++ takes it (the condition is
// promoted to int), so the name stays the C++ compiler's.
func ascii(b byte) string {
	switch b {
	case cstdio.EOF:
		return "never"
	case 'a':
		return "a"
	}
	return "?"
}

func main() {
	fmt.Println(len(buf), len(table), len(table[0]), len(frame), len(word), bufHalf)
	fmt.Printf("%.6f %.4f\n", tau, tenths)
	fmt.Println(big, low, greeting, sep, limit, rows, shape, enumSum, minInt)
	fmt.Println(len(esc), esc[len(esc)-2:])
	var f Frame
	fmt.Println(len(f.cells), len(f.tail), consts.OS_ID, consts.ON)
	fmt.Println(classify(-1), classify(-7), classify(0), classify(1), classify(4), classify(40), classify(9))
	fmt.Println(ascii('a'), ascii('b'), ascii(255))

	x := 1 << consts.SHIFT
	var y uint64 = 3
	y <<= consts.SHIFT
	z := 1000000 >> consts.SHIFT
	fmt.Println(x, y, z)

	// Outside the places that need a constant the macro is the macro: C++'s int, not a Go constant.
	n := consts.NEG
	fmt.Println(n, consts.kScale, consts.TWICE(4), consts.next())
}

// Output:
// 8192 3 4 12 5 4096
// 6.283185 0.3000
// 2147483648 255 hello, world 47 20 12 17 9 -2147483648
// 17 AB
// 4 12 1 true
// eof neg-or-zero neg-or-zero enum enum forty other
// a ? ?
// 32 96 31250
// -7 2.5 8 0
