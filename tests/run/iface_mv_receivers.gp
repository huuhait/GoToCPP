// phase: 05

// The receiver expression of an interface method value (step 5.9): a package variable, a
// field, an element of an array, slice or map, a pointer dereference, a call result, a
// parenthesized expression; the value is read where the method value is made.
package main

import "fmt"

type Greeter interface{ Greet() string }

type G struct{ s string }

func (g G) Greet() string { return g.s }

var global Greeter = G{"global-1"}

type Holder struct {
	g   Greeter
	arr [2]Greeter
	m   map[string]Greeter
}

func (h *Holder) get() Greeter { return h.g }

func main() {
	f1 := global.Greet
	global = G{"global-2"}
	f2 := global.Greet
	global = G{"global-3"}
	fmt.Println(f1(), f2(), global.Greet())

	h := &Holder{g: G{"field-1"}, arr: [2]Greeter{G{"a0"}, G{"a1"}}, m: map[string]Greeter{"k": G{"m1"}}}
	f3 := h.g.Greet
	f4 := h.arr[1].Greet
	f5 := h.m["k"].Greet
	f6 := h.get().Greet
	h.g = G{"field-2"}
	h.arr[1] = G{"a1-new"}
	h.m["k"] = G{"m2"}
	fmt.Println(f3(), f4(), f5(), f6())
	fmt.Println(h.g.Greet(), h.arr[1].Greet(), h.m["k"].Greet(), h.get().Greet())

	sl := []Greeter{G{"s0"}, G{"s1"}}
	i := 0
	f7 := sl[i].Greet
	i = 1
	f8 := sl[i].Greet
	sl[0], sl[1] = sl[1], sl[0]
	fmt.Println(f7(), f8())

	p := &sl[0]
	f9 := (*p).Greet
	*p = G{"replaced"}
	fmt.Println(f9(), (*p).Greet())

	f10 := (global).Greet
	f11 := (h.arr[0]).Greet
	fmt.Println(f10(), f11())

	pp := &h.g
	f12 := (*pp).Greet
	h.g = G{"after"}
	fmt.Println(f12(), (*pp).Greet())

	// A function that returns different interface values.
	n := 0
	mk := func() Greeter { n++; return G{fmt.Sprint("made-", n)} }
	f13 := mk().Greet
	f14 := mk().Greet
	fmt.Println(n, f13(), f14(), f13())

	// The receiver is read before the arguments of a surrounding call are evaluated.
	order := func(a, b string) string { return a + "/" + b }
	cur := Greeter(G{"first"})
	swap := func() string { cur = G{"second"}; return "swapped" }
	fmt.Println(order(cur.Greet(), swap()), cur.Greet())
}

// Output:
// global-1 global-2 global-3
// field-1 a1 m1 field-1
// field-2 a1-new m2 field-2
// s0 s1
// s1 replaced
// global-3 a0
// field-2 after
// 2 made-1 made-2 made-1
// first/swapped second
