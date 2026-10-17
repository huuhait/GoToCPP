// phase: 04

// A closure that uses a C++ object shares it with the function, as Go's closures share a
// variable: a member call through the closure is visible outside (and the other way round), also
// when the closure never assigns the variable and when C++ changes the object through a
// reference (std.swap, a function that takes Counter&). Step 4.8: the object is boxed, because
// capturing a copy would lose those changes.
package main

import (
	"fmt"

	"./objects.h"
	"<utility>"
	"<vector>"
)

func main() {
	// member calls through the closure
	v := std.vector[int]{}
	add := func(x int) { v.push_back(C.int(x)) }
	add(1)
	add(2)
	fmt.Println(int(v.size()))

	// and the other way round: a change in the function is seen by the closure
	size := func() int { return int(v.size()) }
	v.push_back(7)
	fmt.Println(size())

	// a C++ function that changes it through a reference
	var c objects.Counter
	bumpAll := func() { objects.bump(c) }
	bumpAll()
	c.inc()
	bumpAll()
	fmt.Println(int(c.n), int(c.get()))

	// std.swap through a closure: neither variable is assigned
	a, b := std.vector[int]{1, 2, 3}, std.vector[int]{4}
	swap := func() { std.swap(a, b) }
	swap()
	fmt.Println(int(a.size()), int(b.size()))

	// two objects in one closure, and the closure's own declarations
	x, y := std.vector[int]{}, std.vector[int]{}
	fill := func(n int) {
		for i := 0; i < n; i++ {
			x.push_back(C.int(i))
			y.push_back(C.int(i * i))
		}
		sum := 0
		for _, e := range y {
			sum += int(e)
		}
		fmt.Println("sum", sum)
	}
	fill(4)
	fmt.Println(int(x.size()), int(y.size()), int(y.back()))

	// a closure in a closure
	z := std.vector[int]{}
	outer := func() func() {
		z.push_back(1)
		return func() { z.push_back(2) }
	}
	inner := outer()
	inner()
	inner()
	fmt.Println(int(z.size()), int(z[0]), int(z[2]))

	// a C++ number and a raw pointer in a closure
	n := C.int(5)
	p := &c
	step := func() { n += 2; objects.bump_ptr(p) }
	step()
	step()
	fmt.Println(int(n), int(c.n))
}

// Output:
// 2
// 3
// 21 21
// 1 3
// sum 14
// 4 4 9
// 3 1 2
// 9 221
