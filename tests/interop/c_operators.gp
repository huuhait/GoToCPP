// phase: 04

// Operators on C values follow C++'s rules: an untyped constant next to a C value is a C literal
// (7 is an int, 2.5 a double), ^x is ~x, x &^ y is x & ~y, and ++ is C++'s ++x. Conditions must
// be bools, as in Go: a comparison, a C++ bool (v.empty()) or a C bool variable.
package main

import (
	"fmt"

	"<algorithm>"
	"<cmath>"
	"<vector>"
)

func main() {
	var a C.int = 7
	b := a * 3 // a C int: auto
	fmt.Println(int(a+1), int(b-a), int(b/2), int(b%4), int(-a), int(+a), int(^a))
	fmt.Println(int(a<<2), int(b>>1), int(a&3), int(a|8), int(a^5), int(a&^3))
	sh := 3 // a gocpp count: Go allows any integer type for it
	fmt.Println(int(a<<sh), int(a>>sh))
	fmt.Println(bool(a == 7), bool(a != 7), bool(a < b), bool(a <= 7), bool(b > 20), bool(b >= 22))
	a += 3
	a -= 1
	a *= 2
	a /= 3
	a %= 4
	a <<= 4
	a >>= 1
	a |= 1
	a &= 0x7f
	a ^= 2
	a &^= 8
	a++
	a--
	a++
	fmt.Println(int(a))

	d := cmath.sqrt(2.0) * 2
	d -= 0.5
	fmt.Println(int(d*1000), int(cmath.floor(d)+0.5))

	n := 5
	fmt.Println(int(std.max(n, int(0))), int(std.min(C.int(n), a)))
	ok := a > 3 && b != 0
	var v std.vector[C.int]
	if ok && v.empty() {
		fmt.Println("ok and empty")
	}
	v.push_back(a)
	for !v.empty() {
		v.pop_back()
	}
	full := !v.empty()
	if !full || n > 100 {
		fmt.Println("empty again")
	}
	switch {
	case v.size() > 0:
		fmt.Println("not reached")
	case ok:
		fmt.Println("ok")
	}
}

// Output:
// 8 14 10 1 -7 7 -8
// 28 10 3 15 2 4
// 56 0
// true false true true true false
// 20
// 2328 2
// 5 5
// ok and empty
// empty again
// ok
