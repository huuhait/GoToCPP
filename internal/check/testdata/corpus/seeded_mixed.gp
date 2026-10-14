// Seeded mistakes: Go errors inside expressions with C/C++ operands that
// start at the same place, next to the same shapes without a mistake.
package main

import (
	"fmt"
	"unsafe"

	"./geom.h"
	"corpus/clib"
	"<vector>"
)

type Handle geom.handle

type Point geom.point

func main() {
	n, m := 1, 2
	s := "a"
	var h Handle
	var u unsafe.Pointer
	var f float64
	v := std.vector[int]{1}
	fmt.Println(n+m == h, n+int(v.size()) > h)
	fmt.Println(n+s == h)           // ERROR "invalid operation: n \+ s \(mismatched types int and string\)"
	fmt.Println(1 == u && h == 0)   // ERROR "cannot convert 1 \(untyped int constant\) to type unsafe.Pointer"
	fmt.Println(f<<1 + h)           // ERROR "invalid operation: shifted operand f \(variable of type float64\) must be integer"
	fmt.Println(s+n == clib.Origin) // ERROR "invalid operation: s \+ n \(mismatched types string and int\)"
	xs := []int{1, 2}
	fmt.Println(xs[h], xs[s]) // ERROR "invalid argument: index s \(variable of type string\) must be integer"
	hs := []Handle{h}
	hs = append(hs, 1)
	ns := []int{1}
	ns = append(ns, h, "x") // ERROR "cannot use \x22x\x22 \(untyped string constant\) as int value in argument to append"
	fmt.Println(hs, ns)
	pp := &Point{x: 1}
	fmt.Println(pp.x, pp+1) // ERROR "invalid operation: pp \+ 1 \(mismatched types \*Point and untyped int\)"
}
