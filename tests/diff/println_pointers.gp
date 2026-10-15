// phase: 03
// diff: stderr

// The built-ins println and print write a pointer as its address, 0x0 for a nil one (also of a
// named pointer type). A non-nil address differs from run to run, so only nil ones print here.
package main

type T struct{ x int }

type P *int

func main() {
	var p *int
	var q *T
	var r P
	println(p, q, r)
	print(p, "|", q, "\n")
	x := 1
	p = &x
	println(p != nil, p == r)
}
