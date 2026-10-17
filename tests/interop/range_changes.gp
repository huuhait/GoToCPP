// phase: 04

// Changes to a C++ container during a range over it (step 4.7, divergence D21): a vector is
// walked by index with its size read once, as Go reads a slice's length once, so a push_back in
// the body is not visited (and doesn't break the walk when it reallocates); a write to a later
// element is seen. A std::map follows C++'s rules: an insert doesn't invalidate its iterators, so
// a new key after the current one is visited and one before it is not (Go may or may not
// produce a new key). Also labels, break and continue, defer and shadowing in the body.
package main

import (
	"fmt"

	"<map>"
	"<vector>"
)

func walk(v std.vector[C.int]) int {
	sum := 0
	for _, x := range v {
		defer fmt.Println("deferred", int(x)) // the argument is evaluated now
		sum += int(x)
	}
	return sum
}

func main() {
	var v std.vector[C.int]
	v.push_back(1)
	v.push_back(2)
	v.push_back(3)
	fmt.Print("vector")
	for i, x := range v {
		v.push_back(x * 10) // not visited: the size was read once
		if i == 0 {
			v[2] = 30 // a later element: seen
		}
		fmt.Print(" ", int(x))
	}
	fmt.Println("", int(v.size()), int(v[5]))

	var m std.map[C.int, C.int]
	m[2] = 20
	m[4] = 40
	fmt.Print("map")
	for k, e := range m {
		if k == 2 {
			m[1] = 10 // before the current key: not visited
			m[3] = 30 // after it: visited
		}
		fmt.Print(" ", int(k), "=", int(e))
	}
	fmt.Println("", int(m.size()))

	var grid std.vector[std.vector[C.int]]
	grid.resize(3)
	for i := range grid {
		grid[i].push_back(C.int(i))
		grid[i].push_back(C.int(i) + 10)
	}
	fmt.Print("labels")
outer:
	for i, row := range grid {
		for _, x := range row {
			if x == 1 {
				continue outer
			}
			if x == 12 {
				break outer
			}
			fmt.Print(" ", int(i), ":", int(x))
		}
	}
	fmt.Println()

	fmt.Print("shadow")
	for _, x := range v {
		x := int(x) + 1 // shadows the range variable
		if x > 3 {
			break
		}
		fmt.Print(" ", x)
	}
	fmt.Println()
	fmt.Println(walk(v))
}

// Output:
// vector 1 2 30 6 300
// map 2=20 3=30 4=40 4
// labels 0:0 0:10 2:2
// shadow 2 3
// deferred 300
// deferred 20
// deferred 10
// deferred 30
// deferred 2
// deferred 1
// 363
