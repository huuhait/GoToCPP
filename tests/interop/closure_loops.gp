// phase: 04

// C++ objects in closures that outlive an iteration or the function (step 4.8): the variable of
// a 3-clause for loop and of a range over a C++ container is a new variable for each iteration
// (Go 1.22), a parameter and a struct that holds a C++ object are boxed like a local, and a box
// lives as long as a closure that holds it.
package main

import (
	"fmt"

	"./objects.h"
	"<vector>"
)

type Bag struct {
	items std.vector[int]
	name  string
}

func counter() func() int {
	n := C.int(10)
	return func() int {
		n++
		return int(n)
	}
}

func sizeOf(v std.vector[int]) func() int {
	return func() int {
		v.push_back(0) // the closure's own copy of the parameter
		return int(v.size())
	}
}

func bagAdder(name string) (func(int), func() int) {
	var b Bag
	b.name = name
	return func(x int) { b.items.push_back(C.int(x)) }, func() int { return len(b.name) + int(b.items.size()) }
}

func main() {
	next := counter()
	fmt.Println(next(), next())
	other := counter()
	fmt.Println(other(), next())

	// a 3-clause loop's variable is new for each iteration
	var fns []func() int
	for i := C.int(0); i < 3; i++ {
		fns = append(fns, func() int { i += 10; return int(i) })
	}
	for _, f := range fns {
		fmt.Print(f(), " ", f(), "|")
	}
	fmt.Println()

	// and so is the variable of a range over a C++ container
	v := std.vector[int]{5, 6, 7}
	var gets []func() int
	for i, x := range v {
		gets = append(gets, func() int { x++; return int(i)*100 + int(x) })
	}
	for _, g := range gets {
		fmt.Print(g(), " ", g(), "|")
	}
	fmt.Println()

	// the object of a range over a vector of C++ objects
	cs := std.vector[objects.Counter]{}
	for i := 0; i < 3; i++ {
		cs.push_back(objects.Counter{})
	}
	var incs []func()
	for _, c := range cs {
		incs = append(incs, func() { c.add(5) })
	}
	for _, inc := range incs {
		inc()
	}
	fmt.Println(int(cs[0].get()), "(the elements are copies)")

	size := sizeOf(std.vector[int]{1, 2})
	fmt.Println(size(), size())

	add, total := bagAdder("bag")
	add(1)
	add(2)
	fmt.Println(total())
}

// Output:
// 11 12
// 11 13
// 10 20|11 21|12 22|
// 6 7|107 108|208 209|
// 0 (the elements are copies)
// 3 4
// 5
