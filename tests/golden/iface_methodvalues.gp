// phase: 05

// Method values and method expressions of interface methods, and keys that hold interfaces
// (steps 5.9 and 5.6): the receiver of a method value is bound by gp::method_recv (a nil
// interface panics where the value is made), a struct key with an interface field has
// gp_hash_panics, a plain struct key does not.
package main

import "fmt"

type Shape interface{ Area() int }

type Sq struct{ s int }

func (q Sq) Area() int { return q.s * q.s }

type Key struct {
	id  int
	val any
}

type Plain struct{ a, b int }

type Holder struct{ Shape }

func main() {
	var s Shape = Sq{3}
	f := s.Area
	e := Shape.Area
	h := Holder{s}
	g := h.Area
	m := map[Key]int{{1, "a"}: 1}
	p := map[Plain]int{{1, 2}: 1}
	fmt.Println(f(), e(s), g(), m[Key{1, "a"}], p[Plain{1, 2}])
}
