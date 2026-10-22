// phase: 09

// DeepEqual on structures far deeper than the 8 MiB stack of gocpp could recurse through (step
// 9.3 lows): a linked list of 200000 nodes, a slice nested 100000 deep through interfaces, a
// map nested the same way and a list that ends in a cycle. Go grows its stack to 1 GB and
// handles them; the walk of gocpp keeps its own stack on the heap.
package main

import (
	"fmt"
	"reflect"
)

type node struct {
	next *node
	val  int
}

type list []any

type tree struct {
	kids map[string]*tree
}

func makeList(n, last int) *node {
	var head *node
	for i := 0; i < n; i++ {
		v := i
		if i == 0 {
			v = last
		}
		head = &node{head, v}
	}
	return head
}

func makeNest(n int) any {
	var nest any = list(nil)
	for i := 0; i < n; i++ {
		nest = list{nest}
	}
	return nest
}

func makeTree(n int) *tree {
	root := &tree{}
	for i := 0; i < n; i++ {
		root = &tree{map[string]*tree{"k": root}}
	}
	return root
}

// chain unlinks the nodes one by one, so that dropping a long list does not recurse.
func chain(head *node) {
	for head != nil {
		next := head.next
		head.next = nil
		head = next
	}
}

func main() {
	const size = 200000

	a := makeList(size, 7)
	b := makeList(size, 7)
	fmt.Println("lists equal:", reflect.DeepEqual(a, b))
	c := makeList(size, 8)
	fmt.Println("lists differ at the tail:", reflect.DeepEqual(a, c))
	chain(a)
	chain(b)
	chain(c)

	fmt.Println("nests equal:", reflect.DeepEqual(makeNest(100000), makeNest(100000)))
	fmt.Println("nests differ:", reflect.DeepEqual(makeNest(100000), makeNest(99999)))

	fmt.Println("trees equal:", reflect.DeepEqual(makeTree(50000), makeTree(50000)))
	fmt.Println("trees differ:", reflect.DeepEqual(makeTree(50000), makeTree(49999)))

	// a cycle at the end of a long list: the visited pairs stop the walk
	x := makeList(size, 1)
	y := makeList(size, 1)
	tailX := x
	for tailX.next != nil {
		tailX = tailX.next
	}
	tailX.next = x
	tailY := y
	for tailY.next != nil {
		tailY = tailY.next
	}
	tailY.next = y
	fmt.Println("cyclic lists equal:", reflect.DeepEqual(x, y))
	tailX.next = nil
	tailY.next = nil
	chain(x)
	chain(y)
}

// Output:
// lists equal: true
// lists differ at the tail: false
// nests equal: true
// nests differ: false
// trees equal: true
// trees differ: false
// cyclic lists equal: true
