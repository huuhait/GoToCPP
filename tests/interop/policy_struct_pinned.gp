// phase: 04

// A gocpp struct that holds an object C++ can neither copy nor move (a std::mutex, here the
// Lock of objects.h) is built in place wherever Go would build it: a variable, a literal, &S{...},
// new(S), a boxed variable that a closure uses, and a defer on its field. Copying it (t := s) is
// an error (policy_errors_pinned.gp).
package main

import (
	"fmt"

	"./objects.h"
)

type S struct {
	mu objects.Lock
	n  int
}

func variable() {
	var s S
	s.mu.lock()
	defer s.mu.unlock()
	s.n++
	fmt.Println("variable", s.n)
}

func literal() {
	s := S{n: 3}
	f := func() { s.mu.lock(); s.n++; s.mu.unlock() }
	f()
	fmt.Println("literal", s.n)
}

func pointer() {
	s := &S{n: 5}
	t := new(S)
	s.mu.lock()
	defer s.mu.unlock()
	t.mu.lock()
	defer t.mu.unlock()
	fmt.Println("pointer", s.n, t.n)
}

func main() {
	variable()
	literal()
	pointer()
}

// Output:
// variable 1
// literal 4
// pointer 5 0
