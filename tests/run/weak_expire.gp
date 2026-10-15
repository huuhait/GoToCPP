// phase: 03
// divergence: D1

// When a weak pointer's Value becomes nil (D1): in gocpp at once, when the last strong pointer to
// the target's block goes away; in Go only after a garbage collection finds the target
// unreachable (maybe much later, maybe never for tiny objects), so real Go prints true where this
// test expects false. A pointer into a block (a field, an element) keeps the whole block alive; a
// pointer to a package variable never expires; two weak pointers made from the same pointer
// stay equal after the target is gone.
package main

import (
	"fmt"
	"weak"
)

type T struct {
	n    int
	self weak.Pointer[T]
}

type Node struct {
	kids []*Node
}

var global T

func alive(w weak.Pointer[T]) bool { return w.Value() != nil }

func local() weak.Pointer[T] {
	x := T{n: 5} // boxed: its address is taken
	w := weak.Make(&x)
	fmt.Println("inside:", w.Value() != nil, w.Value().n)
	return w
}

func tree(depth int) *Node {
	n := &Node{}
	if depth > 0 {
		n.kids = []*Node{tree(depth - 1), tree(depth - 1)}
	}
	return n
}

func all(n *Node, ws *[]weak.Pointer[Node]) {
	*ws = append(*ws, weak.Make(n))
	for _, k := range n.kids {
		all(k, ws)
	}
}

func count(ws []weak.Pointer[Node]) int {
	c := 0
	for _, w := range ws {
		if w.Value() != nil {
			c++
		}
	}
	return c
}

func main() {
	p := &T{n: 1}
	w := weak.Make(p)
	same := weak.Make(p)
	q := p
	p = nil
	fmt.Println("one left:", w.Value() != nil)
	q = nil
	fmt.Println("none left:", w.Value() != nil, w == same, w == weak.Pointer[T]{})

	r := &T{n: 2}
	r.self = weak.Make(r) // a weak pointer to itself is no cycle
	field := weak.Make(&r.n)
	r = nil
	fmt.Println("self:", field.Value() != nil)

	s := []int{1, 2, 3}
	e := weak.Make(&s[2])
	t := s[1:]
	s = nil
	fmt.Println("element, the array held by t:", e.Value() != nil, *e.Value())
	t = t[:0]
	fmt.Println("t[:0] still holds it:", e.Value() != nil)
	t = nil
	fmt.Println("no slice left:", e.Value() != nil)

	lw := local()
	fmt.Println("after return:", lw.Value() != nil)

	g := weak.Make(&global)
	fmt.Println("global:", g.Value() != nil)

	root := tree(4)
	var ws []weak.Pointer[Node]
	all(root, &ws)
	fmt.Println("tree:", len(ws), count(ws))
	root.kids[0] = nil
	fmt.Println("one subtree gone:", count(ws))
	root = nil
	fmt.Println("all gone:", count(ws))

	cache := map[string]weak.Pointer[T]{}
	keep := &T{n: 7}
	cache["keep"] = weak.Make(keep)
	cache["drop"] = weak.Make(&T{n: 8})
	for _, k := range []string{"drop", "keep", "none"} {
		fmt.Println("cache", k, alive(cache[k]))
	}
	fmt.Println(keep.n, q == nil)
}

// Output:
// one left: true
// none left: false true false
// self: false
// element, the array held by t: true 3
// t[:0] still holds it: true
// no slice left: false
// inside: true 5
// after return: false
// global: true
// tree: 31 31
// one subtree gone: 16
// all gone: 0
// cache drop false
// cache keep true
// cache none false
// 7 true
