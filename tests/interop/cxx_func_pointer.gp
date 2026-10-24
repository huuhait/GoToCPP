// phase: 07

// A pointer to a gocpp func, slice or map of C++ values is a gocpp pointer to the variable (a box),
// as for any gocpp variable, not a raw C++ address: pf := &f with f a func(std.string) int, a field
// *func(std.string) int, (*pf)(x) is a call through it, not a conversion; also in a generic function.
package main

import (
	"fmt"

	"<string>"
)

type P struct{ p *func(std.string) int }

type PS struct{ p *[]std.string }

func G[T any](v T) T {
	f := func(s std.string) T { return v }
	pf := &f
	return (*pf)(std.string("x"))
}

func H[T any](v T) int {
	xs := []std.string{"a"}
	p := &xs
	*p = append(*p, std.string("b"))
	return len(xs)
}

func main() {
	f := func(s std.string) int { return int(s.size()) }
	pf := &f
	fmt.Println((*pf)(std.string("abc")))
	q := P{p: &f}
	fmt.Println((*q.p)(std.string("ab")))
	*pf = func(s std.string) int { return 100 }
	fmt.Println(f(std.string("")), (*q.p)(std.string("")))

	xs := []std.string{"a"}
	ps := PS{p: &xs}
	*ps.p = append(*ps.p, std.string("b"))
	fmt.Println(len(xs), (*ps.p)[1])

	fmt.Println(G(3), G("s"), H(1))
}

// Output:
// 3
// 2
// 100 100
// 2 b
// 3 s 2
