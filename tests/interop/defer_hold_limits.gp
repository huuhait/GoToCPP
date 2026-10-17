// phase: 04

// What the holds of step 4.8 do not see (docs/spec/interop.md, "not tracked"): the address of a
// local of an inner block that a Go function stores (register(&a), add(&r, &a)) is not followed
// into the function, so the local dies with its block, as in C++ (interprocedural escape analysis
// is a TODO). A local of the function's own block is held whatever happens to its address, which is
// what this test uses; and a closure that captures a pointer which holds the address of a loop
// local, stored outside the loop, is followed. No pointer here is used after its local died.
package main

import (
	"fmt"

	"./objects.h"
)

type Reg struct{ ps []*objects.Counter }

var global *objects.Counter

func register(p *objects.Counter)    { global = p }
func add(r *Reg, p *objects.Counter) { r.ps = append(r.ps, p) }

func viaCall() {
	defer func() { fmt.Println("call", int(global.get())) }()
	a := objects.Counter{} // the function's own block: held
	a.add(4)
	register(&a)
}

func viaArgStore() {
	var r Reg
	defer func() { fmt.Println("arg", len(r.ps), int(r.ps[0].get())) }()
	b := objects.Counter{}
	b.add(5)
	add(&r, &b)
}

func viaClosure() {
	var fs []func()
	defer func() {
		for _, f := range fs {
			f()
		}
		fmt.Println("closures done")
	}()
	for i := 0; i < 2; i++ {
		a := objects.Counter{}
		p := &a // a closure that captures p, stored outside the loop: followed
		fs = append(fs, func() { p.inc(); fmt.Println("closure", int(p.get())) })
	}
}

func main() {
	viaCall()
	viaArgStore()
	viaClosure()
}

// Output:
// call 4
// arg 1 5
// closure 1
// closure 1
// closures done
