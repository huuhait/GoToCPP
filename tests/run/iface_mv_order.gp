// phase: 05

// The order of evaluation of an interface method value next to calls (step 5.9): gc reads the
// interface variable after the calls of the same statement, as it does for a method call's
// receiver, so the method value binds the value the call left behind.
package main

import "fmt"

type Greeter interface{ Greet() string }
type G struct{ s string }

func (g G) Greet() string { return g.s }

var cur Greeter = G{"first"}

func swap() int { cur = G{"second"}; return 1 }

func use(f func() string, n int) string { return f() }

func use2(n int, f func() string) string { return f() }

type H struct {
	f func() string
	n int
}

func main() {
	fmt.Println(use(cur.Greet, swap()))
	cur = G{"first"}
	fmt.Println(use2(swap(), cur.Greet))
	cur = G{"first"}
	h := H{cur.Greet, swap()}
	fmt.Println(h.f())
	cur = G{"first"}
	fs := []func() string{cur.Greet, func() string { swap(); return "x" }}
	fmt.Println(fs[0](), fs[1](), cur.Greet())
	cur = G{"first"}
	x, y := cur.Greet, swap()
	fmt.Println(x(), y)
	cur = G{"first"}
	var z func() string
	var w int
	z, w = cur.Greet, swap()
	fmt.Println(z(), w)
}

// Output:
// second
// second
// second
// first x second
// second 1
// second 1
