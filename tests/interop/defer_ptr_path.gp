// phase: 04

// defer s.c.inc() where s is a gocpp pointer to a struct with C++ fields (defer s.mu.unlock()): Go
// evaluates the pointer when the defer statement runs, and the call runs later through it, so the
// frame keeps a copy of the pointer (which also keeps the struct alive), and the call uses the
// struct s pointed to then, also if s points elsewhere by the time the function returns.
package main

import (
	"fmt"

	"./objects.h"
	"<vector>"
)

type S struct {
	c    objects.Counter
	v    std.vector[int]
	name string
}

func (s *S) report() {
	fmt.Println(s.name, int(s.c.get()), int(s.v.size()))
}

func run() {
	a := &S{name: "a"}
	b := &S{name: "b"}
	defer func() { a.report() }() // runs last: after s.c.inc() and s.v.push_back(7)
	s := a
	defer s.c.inc()
	defer s.v.push_back(7)
	defer s.report()
	s = b // the deferred calls above still use a
	defer b.report()
	s.c.add(5)
	defer a.report()
}

func main() {
	run()
}

// Output:
// a 0 0
// b 5 0
// a 0 0
// a 1 1
