// Package variables: constants at compile time (constinit), also array and struct
// literals of number and bool constants; the others in Go's initialization order, a
// variable larger than 128 KiB built in place; then the init functions in source order.
package main

import "fmt"

var a = b + 1 // after b
var b = next("b")
var c, d = pair()
var _ = next("blank")

var (
	n         = 42
	big       = 1 << 62
	s         = "str"
	empty     = ""
	f         = 0.5
	t         = true
	zero      int
	zeroS     string
	counter   = 0
	initCount int
)

const k = 7

type point struct {
	x, y int
	ok   bool
}

type named struct {
	name string
	n    int
}

type grid [2][2]int8

type bigBlock struct {
	cells [1 << 15]int
	tag   string
}

// Constant literals: constinit.
var (
	origin = point{}
	unit   = point{1, 1, true}
	onlyY  = point{y: 2}
	pts    = [3]point{1: {x: 4}}
	cells  = grid{{1, 2}, {3, 4}}
)

// Everything else: gp_var_init, in Go's order.
var (
	nm     = named{"n", n}
	list   = []int{n, k}
	table  = map[string]int{"a": 1}
	pp     = &unit
	scaled = scale(2)
	ref    = func() int { return unit.x + counter }
	huge   = bigBlock{tag: "huge", cells: [1 << 15]int{1: n}}
	hugeA  = [1 << 15]int{n, 2: k}
)

func scale(f int) point {
	return point{unit.x * f, unit.y * f, unit.ok}
}

func next(name string) int {
	counter = counter + 1
	println("init", name, counter)
	return counter * 10
}

func pair() (int, string) {
	return next("pair"), "d"
}

func init() {
	initCount = initCount + 1
	println("first init", a, b)
}

func init() {
	initCount = initCount + 1
	println("second init", initCount)
}

func main() {
	fmt.Println(a, b, c, d, n, big, s, empty, f, t, zero, zeroS, k, initCount)
	n = n + k
	fmt.Println(n)
	fmt.Println(origin.ok, unit.x, onlyY.y, pts[1].x, cells[1][0], nm.name, nm.n, len(list), table["a"])
	pp.x = 10
	fmt.Println(unit.x, scaled.y, ref(), huge.tag, huge.cells[1], hugeA[0], hugeA[2])
}
