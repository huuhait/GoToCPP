// phase: 04

// Reading bit-fields of C structs into variables (x := b.u3), as results, into fields, through
// conversions and through fmt: unsigned, signed, bool, enum and a 40-bit field, from a struct
// value, a pointer and a gocpp struct holding the C struct. A bit-field can't bind to a reference,
// so the helpers that read a member take numbers by value.
package main

import (
	"fmt"

	"./cbitfields.h"
)

type Holder struct {
	b cbitfields.BF
	v C.uint
}

func third(b *cbitfields.BF) C.uint { return b.u3 }

func wide(b cbitfields.BF) C.longlong { return b.wide }

func main() {
	b := cbitfields.bf_make(5, -3, 1, 1<<35)
	x := b.u3
	var y = b.s5
	z := b.flag
	w := b.wide
	e := b.tone
	fmt.Println(x, y, z, w, e == cbitfields.LIGHT)
	fmt.Println(int(b.u3)+1, int(b.s5), float64(b.wide)/2, uint(b.pad))
	fmt.Println(third(&b), wide(b))

	h := Holder{b: b, v: b.u3}
	h.v = h.b.u3
	hp := &h
	u := hp.b.u3
	s := hp.b.s5
	fmt.Println(h.v, u, s)

	b.u3 = 7
	b.s5 = 15
	n := b.u3
	m := b.s5
	fmt.Println(n, m)
}

// Output:
// 5 -3 true 34359738368 true
// 6 -3 1.7179869184e+10 9
// 5 34359738368
// 5 5 -3
// 7 15
