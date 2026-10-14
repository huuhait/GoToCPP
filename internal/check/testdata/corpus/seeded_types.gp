// Seeded mistakes: wrong types used with the typed results of
// conversions of opaque values, and undefined names inside opaque
// template arguments.
package main

import (
	"fmt"
	"strings"

	"<array>"
	"<map>"
	"<string>"
	"<vector>"
)

type Point struct{ X, Y int }

type Box[T any] struct{ v T }

func main() {
	v := std.vector[int]{1, 2}
	n := int(v.size())
	ps2 := []Point{}
	var s string = n // ERROR "cannot use n \(variable of type int\) as string value in variable declaration"
	_ = n + "x"      // ERROR "mismatched types int and untyped string"
	f := float64(v.front())
	var k int = f                      // ERROR "cannot use f \(variable of type float64\) as int value in variable declaration"
	n.grow()                           // ERROR "n.grow undefined \(type int has no field or method grow\)"
	ps := std.vector[Pointt]{}         // ERROR "undefined: Pointt"
	m := std.map[std.string, Undef]{}  // ERROR "undefined: Undef"
	var a std.array[int, sizeN]        // ERROR "undefined: sizeN"
	bs := std.vector[strings.Buildr]{} // ERROR "undefined: strings.Buildr"
	var box std.vector[Box]            // ERROR "cannot use generic type Box\[T any\] without instantiation"
	var arr std.array[int, len(ps2)]   // ERROR "template argument len\(ps2\) \(value of type int\) must be constant"
	var arr2 std.array[int, n]         // ERROR "template argument n \(variable of type int\) must be constant"
	good := std.vector[Point]{{X: 1}}
	fmt.Println(s, k, ps, m, a, bs, good, box, arr, arr2)
	var total int = v.size()
	total = "x" // ERROR "cannot use \x22x\x22 \(untyped string constant\) as int value in assignment"
	fmt.Println(total)
}
