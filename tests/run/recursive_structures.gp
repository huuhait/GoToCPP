// phase: 03

// Recursive data structures with weak back pointers (Phase 03 exit criterion): a tree whose
// nodes point to their parents with weak.Pointer, a doubly linked list whose prev pointers are
// weak, and a graph whose edges back to an earlier node are weak. Only the strong pointers form
// a tree or a chain, so reference counting frees everything at the end: under GOCPP_FLAGS=-asan,
// LeakSanitizer finds no leak. (When the targets are freed is weak_expire's part, D1.)
package main

import (
	"fmt"
	"weak"
)

// --- a tree with weak parent pointers ---

type Tree struct {
	val    int
	parent weak.Pointer[Tree]
	kids   []*Tree
}

func (t *Tree) add(val int) *Tree {
	k := &Tree{val: val, parent: weak.Make(t)}
	t.kids = append(t.kids, k)
	return k
}

func (t *Tree) path() []int {
	var p []int
	for n := t; n != nil; n = n.parent.Value() {
		p = append([]int{n.val}, p...)
	}
	return p
}

func (t *Tree) sum() int {
	s := t.val
	for _, k := range t.kids {
		s += k.sum()
	}
	return s
}

func build(depth, width, val int) *Tree {
	root := &Tree{val: val}
	var grow func(t *Tree, d int)
	grow = func(t *Tree, d int) {
		if d == 0 {
			return
		}
		for i := 1; i <= width; i++ {
			grow(t.add(t.val*10+i), d-1)
		}
	}
	grow(root, depth)
	grow = nil // the closure refers to itself through grow: break that cycle
	return root
}

// --- a doubly linked list with weak prev pointers ---

type Elem struct {
	val  string
	next *Elem
	prev weak.Pointer[Elem]
}

type List struct {
	head *Elem
	tail weak.Pointer[Elem]
	n    int
}

func (l *List) push(v string) {
	e := &Elem{val: v, prev: l.tail}
	if t := l.tail.Value(); t != nil {
		t.next = e
	} else {
		l.head = e
	}
	l.tail = weak.Make(e)
	l.n++
}

func (l *List) remove(e *Elem) {
	prev := e.prev.Value()
	if prev != nil {
		prev.next = e.next
	} else {
		l.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		l.tail = e.prev
	}
	e.next = nil
	l.n--
}

func (l *List) forward() []string {
	var s []string
	for e := l.head; e != nil; e = e.next {
		s = append(s, e.val)
	}
	return s
}

func (l *List) backward() []string {
	var s []string
	for e := l.tail.Value(); e != nil; e = e.prev.Value() {
		s = append(s, e.val)
	}
	return s
}

// --- a graph: strong edges forward, weak edges back ---

type Vertex struct {
	id   int
	out  []*Vertex
	back []weak.Pointer[Vertex]
}

func connect(from, to *Vertex) {
	if to.id > from.id {
		from.out = append(from.out, to)
	} else {
		from.back = append(from.back, weak.Make(to))
	}
}

func reach(v *Vertex, seen map[int]bool) {
	if seen[v.id] {
		return
	}
	seen[v.id] = true
	for _, w := range v.out {
		reach(w, seen)
	}
	for _, w := range v.back {
		if t := w.Value(); t != nil {
			reach(t, seen)
		}
	}
}

func main() {
	root := build(3, 3, 1)
	leaf := root.kids[2].kids[0].kids[1]
	fmt.Println(root.sum(), leaf.path(), leaf.parent.Value().parent.Value() == root.kids[2])
	root.kids[1] = nil // a whole subtree goes
	fmt.Println(root.kids[0].kids[1].path(), leaf.path())

	var l List
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		l.push(s)
	}
	fmt.Println(l.forward(), l.backward(), l.n)
	l.remove(l.head.next.next) // c
	l.remove(l.head)           // a
	l.remove(l.tail.Value())   // e
	fmt.Println(l.forward(), l.backward(), l.n)

	vs := make([]*Vertex, 6)
	for i := range vs {
		vs[i] = &Vertex{id: i}
	}
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {2, 3}, {3, 1}, {4, 5}, {5, 4}, {3, 3}} {
		connect(vs[e[0]], vs[e[1]])
	}
	seen := map[int]bool{}
	reach(vs[3], seen)
	fmt.Println(len(seen), seen[0], seen[4], len(vs[3].back), vs[2].back[0].Value() == vs[0])

	// A long list: freeing it doesn't recurse 100,000 deep.
	var long List
	for i := 0; i < 100000; i++ {
		long.push("x")
	}
	fmt.Println(long.n, long.tail.Value().prev.Value().val)
}

// Output:
// 34129 [1 13 131 1312] true
// [1 11 112] [1 13 131 1312]
// [a b c d e] [e d c b a] 5
// [b d] [d b] 2
// 4 true false 2 true
// 100000 x
