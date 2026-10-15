// phase: 04

// Operands that are C/C++ values are evaluated in Go's order, also where C++ leaves the order
// open (the arguments of a call, the operands of + and *): the calls of trace print in source
// order with both compilers. The object of a member call comes first, and a call that returns a
// C++ reference stays that object when it goes into a temporary (the_ints()[0] = ... changes
// the vector, not a copy). A C global is read after the calls of its statement, like a package
// variable in Go.
package main

import (
	"fmt"

	"./cvals.h"
	"<vector>"
)

func trace(name string, v int) C.int {
	fmt.Println("eval", name)
	return C.int(v)
}

func main() {
	fmt.Println(int(cvals.add_ints(trace("a", 1), trace("b", 2))))
	fmt.Println(int(trace("c", 3) * trace("d", 4)), int(trace("e", 1)+trace("f", 2)*trace("g", 3)))
	cvals.the_ints().push_back(trace("h", 5))
	cvals.the_ints()[0] = trace("i", 6)
	cvals.the_ints()[0] += trace("j", 1)
	cvals.the_ints()[trace("k", 0)]++
	fmt.Println(int(cvals.the_ints()[0]), int(cvals.the_ints().size()))
	if trace("l", 0) != 0 && trace("m", 1) != 0 {
		fmt.Println("not reached")
	}
	if trace("n", 0) == 0 || trace("o", 1) != 0 {
		fmt.Println("or")
	}
	v := std.vector[C.int](3, 0)
	v[trace("p", 1)] = trace("q", 9)
	fmt.Println(int(v[1]), int(v.at(trace("r", 1))+trace("s", 1)))
	fmt.Println(int(cvals.counter), int(cvals.bump()), int(cvals.counter))
	copied := cvals.the_ints() // a call's result is a value: a copy of the vector
	copied.push_back(1)
	fmt.Println(int(copied.size()), int(cvals.the_ints().size()))
}

// Output:
// eval a
// eval b
// 3
// eval c
// eval d
// eval e
// eval f
// eval g
// 12 7
// eval h
// eval i
// eval j
// eval k
// 8 1
// eval l
// eval n
// or
// eval p
// eval q
// eval r
// eval s
// 9 10
// 1 1 1
// 2 1
