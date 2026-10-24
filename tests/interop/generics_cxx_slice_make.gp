// phase: 07

// An S ~[]E that C++ deduces (a []std.string argument) in make(S, n) and make(S, n, c), in
// append(s, make(S, n)...) (one allocation, as slices.Insert does it), in copy, and in conversions:
// S(nil), S(t) and []E(s) are those of the slice []E, which S is in C++.
package main

import (
	"fmt"

	"<string>"
)

func Grow[S ~[]E, E any](s S, n int) S {
	g := append(s[:len(s):len(s)], make(S, n)...)
	return g[:len(s)]
}

func Fill[S ~[]E, E any](s S, v E) S {
	t := make(S, len(s), 2*len(s)+1)
	copy(t, s)
	for i := range t {
		if i%2 == 1 {
			t[i] = v
		}
	}
	return t
}

func Conv[S ~[]E, E any](s S, v E) (S, []E, int) {
	u := S(nil)
	fmt.Println(u == nil, len(u))
	u = append(u, s...)
	w := S([]E{v})
	n := copy(u, w)
	return append(u, S(w)...), []E(s), n
}

func show[S ~[]E, E any](s S) {
	fmt.Print(len(s), ":")
	for _, x := range s {
		fmt.Print(" ", x)
	}
	fmt.Println()
}

func main() {
	c := []std.string{"a", "b", "c"}
	g := Grow(c, 5)
	fmt.Println(len(g), cap(g) >= 8)
	show(g)
	show(Fill(c, std.string("x")))
	u, e, n := Conv(c, std.string("q"))
	show(u)
	show(e)
	fmt.Println(n)
}

// Output:
// 3 true
// 3: a b c
// 3: a x c
// true 0
// 4: q b c q
// 3: a b c
// 1
