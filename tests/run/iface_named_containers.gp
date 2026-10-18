// phase: 05

// Named slice, map, array and pointer types over named structs in an interface (step 5.6/5.9
// review): the descriptor of the container points at the element's, whichever is declared or
// first converted first, also for types that refer to each other through slices, maps and
// pointers.
package main

import "fmt"

// Declared after use, in the order the program converts them.
func main() {
	var a any = Items{{1}, {2}}
	fmt.Println(len(a.(Items)), a.(Items)[1].v)

	var l Lener = Stack{{1}, {2}, {3}}
	fmt.Println(l.Len())

	var pb any = PB(&Base{7})
	fmt.Println((*pb.(PB)).n)
	var m any = map[string]Base{"x": {5}}
	fmt.Println(m.(map[string]Base)["x"].n)
	var arr any = [2]Base{{1}, {2}}
	fmt.Println(arr.([2]Base)[1].n)
	var named any = NamedMap{"k": {9}}
	fmt.Println(named.(NamedMap)["k"].n)
	var na any = NamedArr{{3}, {4}}
	fmt.Println(na.(NamedArr)[0].n, na == any(NamedArr{{3}, {4}}))

	// Types that refer to each other.
	var x any = A{B{nil}, B{A{}}}
	fmt.Println(len(x.(A)))
	var y any = Tree{"root", []Tree{{"leaf", nil}}}
	fmt.Println(y.(Tree).kids[0].name)
	var z any = Graph{"a": {"b": {}}}
	fmt.Println(len(z.(Graph)["a"]))
	var w any = &Node{val: 1, next: &Node{val: 2}}
	fmt.Println(w.(*Node).next.val)
	var l2 Lener = NodeList{{val: 1}, {val: 2}}
	fmt.Println(l2.Len())
	var ring any = Ring{}
	_, ok := ring.(Ring)
	fmt.Println(ok)
	_, ok = ring.(Items)
	fmt.Println(ok)
}

type Base struct{ n int }
type PB *Base
type NamedMap map[string]Base
type NamedArr [2]Base

type Item struct{ v int }
type Items []Item

type Stack []Item

func (s Stack) Len() int { return len(s) }

type Lener interface{ Len() int }

type A []B
type B struct{ a A }

type Tree struct {
	name string
	kids []Tree
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

// Output:
// 2 2
// 3
// 7
// 5
// 2
// 9
// 3 true
// 2
// leaf
// 1
// 2
// 2
// true
// false
