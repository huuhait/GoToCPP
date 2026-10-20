// phase: 08

// A conversion (*U)(g) is not a temporary: the pointer it converts is still held by the package variable that the callee drops (ADR-086).

package main

import "fmt"

type T struct {
	n    int
	name string
}

type U T

var g *T
var gu *U

func junk() int {
	t := 0
	for i := 0; i < 100; i++ {
		x := &T{n: i, name: "junkjunkjunk"}
		t += x.n
	}
	return t
}

func useU(u *U) int {
	g = nil
	gu = nil
	junk()
	return u.n + len(u.name)
}

func (u *U) m() int {
	g = nil
	gu = nil
	junk()
	return u.n + len(u.name)
}

func useT(t *T) int {
	g = nil
	junk()
	return t.n + len(t.name)
}

func main() {
	g = &T{n: 1, name: "one-padding"}
	fmt.Println(useU((*U)(g)))
	g = &T{n: 2, name: "two-padding"}
	fmt.Println((*U)(g).m())
	g = &T{n: 3, name: "three-padding"}
	fmt.Println(useT((*T)(g)))
	gu = &U{n: 4, name: "four-padding"}
	fmt.Println(useT((*T)(gu)))
	g = &T{n: 5, name: "five-padding"}
	fmt.Println(useU((*U)((*T)(g))))
}

// Output:
// 12
// 13
// 16
// 16
// 17
