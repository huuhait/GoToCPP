// phase: 04
// ExitCode: 2
// Stderr: (?m)^panic: boom in many$

// The holds of step 4.8 also outlive the deferred calls that a panic runs: the closure deferred
// before the locals reads them while the panic goes up.
package main

import (
	"fmt"

	"./objects.h"
)

func many() {
	defer fmt.Println("many done")
	var ps [3]*objects.Counter
	defer func() { fmt.Println("sum", int(ps[0].get())+int(ps[1].get())+int(ps[2].get())) }()
	a, b, c := objects.Counter{}, objects.Counter{}, objects.Counter{}
	ps[0], ps[1], ps[2] = &a, &b, &c
	for i, p := range ps {
		p.add(C.int(i + 1))
	}
	for i := 0; i < 2; i++ {
		x := objects.Counter{}
		x.add(10)
		ps[0] = &x // held: stored in a variable of the outer block (the last one stays)
	}
	panic("boom in many")
}

func main() {
	many()
}

// Output:
// sum 15
// many done
