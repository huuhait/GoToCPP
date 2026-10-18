// phase: 07

// Three packages: b instantiates a's generic functions with its own types, c with both packages' types, and main uses all of them.
package main

import (
	"fmt"

	"example.com/gchain/a"
	"example.com/gchain/b"
	"example.com/gchain/c"
)

func main() {
	fmt.Println(a.Double(21), a.Double(1.25), a.Double(b.Feet(4)), a.Double(a.Meters(2.5)))
	fmt.Println(b.Quad(3), b.Quad(b.Feet(5)), b.FeetQuad(7), b.Pipeline(1.5))
	fmt.Println(c.Both(2, 3))
	fmt.Println(b.Add("x", func(s string) string { return s + "!" }), b.Add(b.Feet(1), a.Double[b.Feet]))
	fmt.Printf("%T %T %T\n", a.Double(b.Feet(1)), b.Quad(a.Meters(1)), b.Add[any](nil, func(v any) any { return v }))
}


// Output:
// 42 2.5 8 5
// 12 20 28 12
// 4 12
// x! 2
// b.Feet a.Meters <nil>
