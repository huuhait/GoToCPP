// phase: 05

// An interface whose method returns a struct that holds the interface: legal in Go, and the
// reason the interface's wrapper methods are defined after all the types (step 5.4).
package main

import "fmt"

type Shape interface {
	Bounds() Rect
	Name() string
}

type Rect struct {
	W, H  int
	Owner Shape
}

type Box struct{ w, h int }

func (b Box) Bounds() Rect { return Rect{b.w, b.h, b} }
func (b Box) Name() string { return "box" }

type Tree interface {
	Kids() []Tree
	Val() int
}

type leaf int
type node struct {
	v    int
	kids []Tree
}

func (l leaf) Kids() []Tree  { return nil }
func (l leaf) Val() int      { return int(l) }
func (n *node) Kids() []Tree { return n.kids }
func (n *node) Val() int     { return n.v }

func sum(t Tree) int {
	s := t.Val()
	for _, k := range t.Kids() {
		s += sum(k)
	}
	return s
}

func main() {
	var s Shape = Box{3, 4}
	r := s.Bounds()
	fmt.Println(r.W*r.H, r.Owner.Name(), r.Owner.Bounds().W)
	t := &node{1, []Tree{leaf(2), &node{3, []Tree{leaf(4), leaf(5)}}, leaf(6)}}
	fmt.Println(sum(t))
}

// Output:
// 12 box 3
// 21
