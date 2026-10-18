// phase: 08

// select and range over channels of C/C++ values (steps 8.5 and 8.6): a std::string variable is copied
// into a send case and a temporary is moved, a std::unique_ptr (move only) travels through select and
// range loops, a received value is bound to a case variable of the C++ type, the case that is not
// chosen does not consume its value, and a C number channel works in a select.
package main

import (
	"fmt"

	"./chanvals.h"
	"<memory>"
	"<string>"
	"<vector>"
)

func main() {
	words := make(chan std.string, 2)
	empty := make(chan std.string, 1)
	empty <- std.string("full") // so that only the first case can happen
	s := std.string("go")
	select {
	case words <- s: // a variable: copied
	case empty <- chanvals.shout(s): // a temporary: evaluated, moved only if chosen
	default:
		fmt.Println("both full")
	}
	s.append("lang")
	select {
	case w, ok := <-words:
		fmt.Println(string(w), ok, string(s))
	}

	// a move-only element in a send case, a receive case and a range loop
	owned := make(chan std.unique_ptr[C.int], 2)
	p := std.make_unique[C.int](42)
	select {
	case owned <- std.move(p):
	}
	fmt.Println(p == nil)
	owned <- std.make_unique[C.int](43)
	close(owned)
	select {
	case q := <-owned:
		fmt.Println(int(*q))
	}
	for q := range owned {
		fmt.Println(int(*q))
	}

	// C numbers
	nums := make(chan C.int, 1)
	idle := make(chan C.int)
	select {
	case nums <- chanvals.twice(21):
	case <-idle:
	}
	select {
	case n := <-nums:
		fmt.Println(int(n))
	case <-idle:
	}

	// a vector in a range loop: the receiver has a copy
	vecs := make(chan std.vector[C.int], 2)
	var xs std.vector[C.int]
	xs.push_back(1)
	vecs <- xs
	xs.push_back(2)
	vecs <- xs
	close(vecs)
	for v := range vecs {
		fmt.Println(int(v.size()), int(xs.size()))
	}

	// the receive of a closed channel of C++ values in a select, with and without the value
	closed := make(chan std.string)
	close(closed)
	var ok bool
	select {
	case _, ok = <-closed:
	}
	fmt.Println(ok)
	select {
	case w, ok := <-closed:
		fmt.Println(string(w) == "", ok)
	}
}

// Output:
// go true golang
// true
// 42
// 43
// 42
// 1 2
// 2 2
// false
// true false
