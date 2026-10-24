// phase: 07

// An API of another package with C++ values: a function with a C++ parameter (lib.Len) and as a
// value, a method with one (n.Call), a func field (n.F), exported package variables of C++ parts
// (lib.Ws, lib.It, with len, append, slicing and an assignment), iterators declared without a type
// and with one (lib.Iter, lib.IterT), and the instance of a generic struct of lib with a func field
// whose C++ type the declaring package's type expression gives (lib.H[int]).
package main

import (
	"fmt"

	"example.com/cxxapi/lib"

	"<string>"
)

func main() {
	n := lib.NH{F: func(s std.string) int { return int(s.size()) }}
	fmt.Println(n.F(std.string("abc")), n.Call(std.string("ab")), lib.Len(std.string("abcd")))
	f := lib.Len
	fmt.Println(f(std.string("q")))
	fmt.Println(len(lib.Ws), lib.Ws[0], int(lib.It.size()), len(lib.Words))
	lib.Ws = append(lib.Ws, std.string("c"))
	fmt.Println(len(lib.Ws), lib.Ws[1:])
	for s := range lib.Iter {
		fmt.Println("iter", s)
	}
	for s := range lib.IterT {
		fmt.Println("typed", s)
	}
	h := lib.H[int]{F: func(s std.string) int { return int(s.size()) * 10 }}
	fmt.Println(h.F(std.string("abc")), h.Call(std.string("ab")))
	var z lib.H[string]
	fmt.Println(z.F == nil, z.N == "")
}

// Output:
// 3 2 4
// 1
// 2 a 3 2
// 3 [b c]
// iter a
// iter b
// typed a
// typed b
// 30 20
// true true
