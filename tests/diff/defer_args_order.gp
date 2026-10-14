// phase: 03

// The function and the arguments of a deferred call are evaluated at the defer statement, in
// gc's order for a call: the calls first, left to right, then the variables, package
// variables, elements and pointers are read; nothing of it runs again when the deferred call
// runs.
package main

import "fmt"

var gx = 1
var gs = []int{1, 2, 3}

func f(tag string) int {
	gx++
	gs[0] *= 10
	fmt.Println("f", tag, gx)
	return gx
}

func inc(p *int) int {
	*p++
	return *p
}

func show(tag string, a, b, c int) { fmt.Println("show", tag, a, b, c) }

func pair() (int, int) {
	fmt.Println("pair")
	return gx, gx * 2
}

func two(a, b int) { fmt.Println("two", a, b) }

func main() {
	defer show("globals", gx, f("1"), gs[0])
	x := 5
	p := &x
	defer show("pointer", *p, inc(p), x)
	defer show("nested", f("2"), inc(&x)+f("3"), *p)
	defer two(pair())
	defer fmt.Println("fmt", gx, f("4"), gs[0], x)
	defer println("println", gx, x)
	m := map[string]int{"k": 1}
	defer fmt.Println("map", m["k"], inc(p))
	m["k"] = 100
	for i := range 2 {
		defer show("loop", i, f("loop"), x)
	}
	gx, x = 1000, 2000
	gs[0] = -1
	fmt.Println("body end")
}
