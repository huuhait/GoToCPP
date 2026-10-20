// phase: 08

// A callee that calls a function without a body to analyse (fmt.Sprint here) is not pure: the
// native calls the String method of its argument, which drops the heap holder of the pointer that
// the caller lent. There is no goroutine in the program, so only the purity decides (ADR-086).
package main

import "fmt"

type T struct {
	n    int
	name string
}

type Holder struct{ p *T }

var h = &Holder{}

func junk() int {
	t := 0
	for i := 0; i < 100; i++ {
		x := &T{n: i, name: "junkjunkjunk"}
		t += x.n
	}
	return t
}

type S struct{}

func (S) String() string {
	h.p = nil
	junk()
	return "dropped"
}

func rel(t *T, o fmt.Stringer) int {
	s := fmt.Sprint(o)
	return t.n + len(t.name) + len(s)
}

func (t *T) relm(o fmt.Stringer) int {
	s := fmt.Sprint(o)
	return t.n + len(t.name) + len(s)
}

func main() {
	h.p = &T{n: 1, name: "object-with-a-long-name"}
	fmt.Println(rel(h.p, S{}))
	h.p = &T{n: 2, name: "another-long-name"}
	fmt.Println(h.p.relm(S{}))
}

// Output:
// 31
// 26
