// phase: 04

// The holds of step 4.8 (the boxes of C++ locals whose address is taken) are released after every
// deferred call, also one that was registered before the local was declared: here a closure
// deferred before the loop reads the objects that the loop's range variables held.
package main

import (
	"fmt"

	"./objects.h"
	"<vector>"
)

var saved *objects.Counter

// A parameter whose address goes to a package variable.
func param(c objects.Counter) {
	defer func() { fmt.Println("param deferred", int(saved.get())) }()
	saved = &c
	c.add(3)
}

// The closure is deferred before the range variables exist.
func rng() {
	vs := std.vector[objects.Counter]{objects.Counter{}, objects.Counter{}}
	var ps []*objects.Counter
	defer func() { fmt.Println("rng deferred", int(ps[0].get()), int(ps[1].get())) }()
	for _, c := range vs {
		c.inc()
		c.inc()
		ps = append(ps, &c)
	}
	fmt.Println("rng body")
}

func main() {
	param(objects.Counter{})
	rng()
}

// Output:
// param deferred 3
// rng body
// rng deferred 2 2
