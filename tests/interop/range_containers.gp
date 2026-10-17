// phase: 04

// range over C++ containers and C arrays (step 4.7): the index walk of the random-access ones
// (vector, deque, array, string, span, views), the iterator walk of lists, sets and maps (one
// variable is the key), C arrays from a header, the four forms with := and with =, and the
// values are copies, as in Go. A std::string gives its bytes as C chars, not runes.
package main

import (
	"fmt"

	"./crange.h"
	"<array>"
	"<deque>"
	"<list>"
	"<map>"
	"<ranges>"
	"<set>"
	"<span>"
	"<string>"
	"<unordered_map>"
	"<vector>"
)

func main() {
	var v std.vector[int]
	v.push_back(5)
	v.push_back(2)
	v.push_back(9)
	fmt.Print("vector")
	for i, x := range v {
		x *= 10 // a copy: v doesn't change
		fmt.Print(" ", int(i), ":", int(x))
	}
	fmt.Println("", int(v[0]))
	fmt.Print("keys")
	for i := range v {
		fmt.Print(" ", int(i))
	}
	n := 0
	for range v {
		n++
	}
	for _, x := range v {
		fmt.Print(" ", int(x))
	}
	fmt.Println("", n)

	// With =, into variables that exist (also a member of a C++ value, a gocpp slice's element).
	var i C.long
	var x C.long
	for i, x = range v {
	}
	fmt.Println(int(i), int(x))
	var last std.array[C.long, 1]
	for _, last[0] = range v {
	}
	xs := make([]C.long, 3)
	for i, xs[i] = range v {
	}
	fmt.Println(int(last[0]), int(xs[0]), int(xs[1]), int(xs[2]))
	for i = range v {
	}
	fmt.Println(int(i))
	// A gocpp array of C values (Go's range) into a C variable that exists.
	var arr [3]C.long
	arr[1] = 8
	n = 0
	for n, x = range arr {
		if n == 1 {
			break
		}
	}
	fmt.Println(n, int(x))

	var d std.deque[C.int]
	d.push_back(1)
	d.push_front(0)
	var a std.array[C.double, 3]
	a[1] = 1.5
	fmt.Print("deque")
	for i, e := range d {
		fmt.Print(" ", int(i), "=", int(e))
	}
	for i, e := range a {
		fmt.Print(" ", int(i), "=", float64(e))
	}
	fmt.Println()

	s := std.string("hey")
	fmt.Print("string")
	for i, c := range s {
		fmt.Print(" ", int(i), string(rune(c)), int(c))
	}
	fmt.Println()

	fmt.Print("table")
	for i, t := range crange.table {
		fmt.Print(" ", int(i), "=", int(t))
	}
	fmt.Println()
	fmt.Print("grid")
	for i := range crange.grid {
		fmt.Print(" ", int(crange.grid[i][2]))
	}
	sp := std.span[C.int](crange.table)
	for i, t := range sp {
		sp[i] = t + 1 // the span's elements are the table's
	}
	fmt.Println("", int(crange.table[0]), int(crange.table[4]))

	fmt.Print("views")
	for _, k := range std.views.iota(3, 6) {
		fmt.Print(" ", int(k))
	}
	for i, x := range std.views.reverse(v) {
		fmt.Print(" ", int(i), "=", int(x))
	}
	for i, x := range std.views.take(v, 2) {
		fmt.Print(" ", int(i), "=", int(x))
	}
	fmt.Println()

	var bs std.vector[bool]
	bs.push_back(true)
	bs.push_back(false)
	fmt.Print("bools")
	for i, b := range bs {
		fmt.Print(" ", int(i), "=", bool(b))
	}
	fmt.Println()

	var l std.list[std.string]
	l.push_back(std.string("x"))
	l.push_back(std.string("yy"))
	fmt.Print("list")
	for i, e := range l {
		e += "!" // a copy
		fmt.Print(" ", int(i), "=", int(e.size()))
	}
	fmt.Println("", int(l.front().size()))

	var set std.set[C.int]
	set.insert(30)
	set.insert(10)
	set.insert(20)
	fmt.Print("set")
	for i, e := range set {
		fmt.Print(" ", int(i), "=", int(e))
	}
	fmt.Println()

	var m std.map[std.string, C.int]
	m["b"] = 2
	m["a"] = 1
	m["c"] = 3
	fmt.Print("map")
	for k, e := range m {
		e++ // a copy
		fmt.Print(" ", int(k.size()), string(rune(k[0])), "=", int(e))
	}
	for k := range m {
		fmt.Print(" ", string(rune(k[0])))
	}
	var key std.string
	var val C.int
	for key, val = range m {
	}
	fmt.Println("", int(m["a"]), string(rune(key[0])), int(val))

	var um std.unordered_map[C.int, std.vector[C.int]]
	um[7].push_back(70)
	um[7].push_back(71)
	for k, e := range um {
		fmt.Println(int(k), int(e.size()), int(e[1]))
	}
}

// Output:
// vector 0:50 1:20 2:90 5
// keys 0 1 2 5 2 9 3
// 2 9
// 9 2 9 5
// 2
// 1 8
// deque 0=0 1=1 0=0 1=1.5 2=0
// string 0h104 1e101 2y121
// table 0=10 1=20 2=30 3=40 4=50
// grid 3 6 11 51
// views 3 4 5 0=9 1=2 2=5 0=5 1=2
// bools 0=true 1=false
// list 0=2 1=3 1
// set 0=10 1=20 2=30
// map 1a=2 1b=3 1c=4 a b c 1 c 3
// 7 2 71
