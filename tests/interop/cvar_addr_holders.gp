// phase: 05

// Two variables that hold the same address (q := &x; ho.p = q, with ho declared outside the loop): both
// are classes that keep &x, whichever the analysis visits first (ADR-139).
package main

import (
	"fmt"
)

import "./cvar_addr_holders.hpp"

type H struct{ p *C.int }

func clobber() int {
	var a [64]int
	for i := range a {
		a[i] = i + 1000
	}
	return a[5]
}

func iv(p *C.int) int {
	return int(*p)
}

func pairs() int {
	var ho0 H
	var ho1 H
	var ho2 H
	var ho3 H
	var ho4 H
	var ho5 H
	var ho6 H
	var ho7 H
	var ho8 H
	var ho9 H
	var ho10 H
	var ho11 H
	var ho12 H
	var ho13 H
	var ho14 H
	var ho15 H
	var ho16 H
	var ho17 H
	var ho18 H
	var ho19 H
	var ho20 H
	var ho21 H
	var ho22 H
	var ho23 H
	for i := 0; i < 2; i++ {
		var x0 C.int = C.int(0)
		q0 := &x0
		ho0.p = q0
		_ = q0
		var x1 C.int = C.int(1)
		q1 := &x1
		ho1.p = q1
		_ = q1
		var x2 C.int = C.int(2)
		q2 := &x2
		ho2.p = q2
		_ = q2
		var x3 C.int = C.int(3)
		q3 := &x3
		ho3.p = q3
		_ = q3
		var x4 C.int = C.int(4)
		q4 := &x4
		ho4.p = q4
		_ = q4
		var x5 C.int = C.int(5)
		q5 := &x5
		ho5.p = q5
		_ = q5
		var x6 C.int = C.int(6)
		q6 := &x6
		ho6.p = q6
		_ = q6
		var x7 C.int = C.int(7)
		q7 := &x7
		ho7.p = q7
		_ = q7
		var x8 C.int = C.int(8)
		q8 := &x8
		ho8.p = q8
		_ = q8
		var x9 C.int = C.int(9)
		q9 := &x9
		ho9.p = q9
		_ = q9
		var x10 C.int = C.int(10)
		q10 := &x10
		ho10.p = q10
		_ = q10
		var x11 C.int = C.int(11)
		q11 := &x11
		ho11.p = q11
		_ = q11
		var x12 C.int = C.int(12)
		q12 := &x12
		ho12.p = q12
		_ = q12
		var x13 C.int = C.int(13)
		q13 := &x13
		ho13.p = q13
		_ = q13
		var x14 C.int = C.int(14)
		q14 := &x14
		ho14.p = q14
		_ = q14
		var x15 C.int = C.int(15)
		q15 := &x15
		ho15.p = q15
		_ = q15
		var x16 C.int = C.int(16)
		q16 := &x16
		ho16.p = q16
		_ = q16
		var x17 C.int = C.int(17)
		q17 := &x17
		ho17.p = q17
		_ = q17
		var x18 C.int = C.int(18)
		q18 := &x18
		ho18.p = q18
		_ = q18
		var x19 C.int = C.int(19)
		q19 := &x19
		ho19.p = q19
		_ = q19
		var x20 C.int = C.int(20)
		q20 := &x20
		ho20.p = q20
		_ = q20
		var x21 C.int = C.int(21)
		q21 := &x21
		ho21.p = q21
		_ = q21
		var x22 C.int = C.int(22)
		q22 := &x22
		ho22.p = q22
		_ = q22
		var x23 C.int = C.int(23)
		q23 := &x23
		ho23.p = q23
		_ = q23
	}
	clobber()
	n := 0
	n += iv(ho0.p)
	n += iv(ho1.p)
	n += iv(ho2.p)
	n += iv(ho3.p)
	n += iv(ho4.p)
	n += iv(ho5.p)
	n += iv(ho6.p)
	n += iv(ho7.p)
	n += iv(ho8.p)
	n += iv(ho9.p)
	n += iv(ho10.p)
	n += iv(ho11.p)
	n += iv(ho12.p)
	n += iv(ho13.p)
	n += iv(ho14.p)
	n += iv(ho15.p)
	n += iv(ho16.p)
	n += iv(ho17.p)
	n += iv(ho18.p)
	n += iv(ho19.p)
	n += iv(ho20.p)
	n += iv(ho21.p)
	n += iv(ho22.p)
	n += iv(ho23.p)
	return n
}

func main() {
	fmt.Println(pairs())
}

// Output:
// 276
