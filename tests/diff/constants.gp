// phase: 02

// Constants: iota with skips, expressions and several names per line,
// typed and untyped constants, exact big values that only fit after the
// expression is folded, and constant conversions.
package main

import "fmt"

const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
	TB
	PB
	EB
)

const (
	pa, pb = iota + 1, iota * 2
	pc, pd
	_, _
	pe, pf
)

const huge = 1 << 200
const fit = huge >> 190
const third = 1.0 / 3
const maxU64 = 1<<64 - 1

func main() {
	fmt.Println(KB, MB, GB, TB, PB, EB)
	fmt.Println(pa, pb, pc, pd, pe, pf)
	fmt.Println(fit, huge/(1<<195), third, float32(third))
	fmt.Println(uint64(maxU64), maxU64>>1, int64(maxU64>>1))
	const (
		x int8 = -1 << 7
		y      = x + 1
	)
	fmt.Println(x, y, -(x + 1))
	const s = "con" + "cat"
	fmt.Println(s, len(s), s[1], s < "conc")
	const big = 1e400 / 1e390
	fmt.Println(big, 'x', 'x'+1, '\n', '\u00e9')
	const r rune = 'z' - 'a'
	fmt.Println(r, r*2, float64(r)/2)
	const mask uint16 = 0xFFFF
	fmt.Println(mask, mask>>8, ^mask&0, mask&^0xFF)
	const neg = -7
	fmt.Println(neg/2, neg%2, neg>>1, uint8(neg&0xFF))
	const fl = 2.5e-3
	fmt.Println(fl, fl*1000, int(fl*1000+0.5))
}
