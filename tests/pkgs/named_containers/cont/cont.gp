package cont

type Lener interface{ Len() int }

// Containers over structs of this package, declared after use and referring to each other.
func Boxes() []any {
	return []any{Items{{1}, {2}}, Stack{{1}, {2}, {3}}, PB(&Base{7}), map[string]Base{"x": {5}},
		[2]Base{{1}, {2}}, NamedMap{"k": {9}}, NamedArr{{3}, {4}}, A{B{nil}, B{A{}}},
		Tree{"root", []Tree{{"leaf", nil}}}, Graph{"a": {"b": {}}}, &Node{val: 1, next: &Node{val: 2}},
		NodeList{{val: 1}, {val: 2}}, Ring{}}
}

type Base struct{ N int }
type PB *Base
type NamedMap map[string]Base
type NamedArr [2]Base

type Item struct{ V int }
type Items []Item
type Stack []Item

func (s Stack) Len() int { return len(s) }

type A []B
type B struct{ a A }

type Tree struct {
	Name string
	Kids []Tree
}

type Graph map[string]map[string]Edge
type Edge struct{ g Graph }

type Node struct {
	val  int
	next *Node
}
type NodeList []Node

func (n NodeList) Len() int { return len(n) }

type Ring [1]RingMap
type RingMap map[int]Ring
