// phase: 03

// A struct of 16 MiB, larger than the C++ stack of the main thread: a new variable and &T{...}
// are made in their blocks (the zero value, then the fields of the literal, in Go's order), and
// a parameter is passed by reference and copied into a box, so nothing of that size is on the
// stack.
package main

import "fmt"

type Big struct {
	cells [1 << 21]int64
	n     int
	name  string
}

var calls int

func next() int {
	calls++
	return calls
}

func total(b Big) int64 {
	b.cells[0] += 100 // the parameter is a copy
	return b.cells[0] + b.cells[1] + int64(b.n)
}

func main() {
	p := &Big{n: 3, name: "p"}
	c := Big{name: "c", n: next()}
	var z Big
	d := Big{[1 << 21]int64{1: 7}, next(), "d"}
	p.cells[1] = 5
	c.cells[1] = 6
	fmt.Println(p.n, p.name, c.n, c.name, z.n, d.n, d.name, d.cells[1])
	fmt.Println(total(*p), total(c), total(d), p.cells[0], len(z.cells))
}

// Output:
// 3 p 1 c 0 2 d 7
// 108 107 109 0 2097152
