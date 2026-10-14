// Opaque values of another package: variables, results of functions with
// C result types, named types over C types and Go structs with C fields.
package main

import (
	"fmt"

	"corpus/clib"
)

func main() {
	fmt.Println(clib.Origin.x, *clib.Rect, clib.Rect.min.x)
	for i, v := range clib.Values {
		fmt.Println(i, v)
	}
	clib.Values.push_back(4)
	n := int(clib.Values.size()) + 1
	fmt.Println(n, clib.Values[0], -clib.Values[1])
	r := clib.New()
	r.max = clib.Origin
	fmt.Println(r.max.x, (*r).min.y)
	for _, row := range clib.Grid() {
		fmt.Println(int(row.size()))
	}
	h := clib.Open("x")
	if h == 0 || h+1 > 10 {
		h++
	}
	p := clib.Point{x: 1, y: 2}
	p.x *= 2
	pair := clib.Pair{N: 1}
	pair.A = clib.Origin
	fmt.Println(h, p.x, pair.A.x, pair.N+1)
	f := clib.New
	fmt.Println(f().min.x)
}
