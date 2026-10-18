// phase: 08

// The classic "equivalent binary trees": walk two trees into channels and compare them with range
// loops.
package main

import "fmt"

type Tree struct {
	Left  *Tree
	Value int
	Right *Tree
}

func insert(t *Tree, v int) *Tree {
	if t == nil {
		return &Tree{nil, v, nil}
	}
	if v < t.Value {
		t.Left = insert(t.Left, v)
	} else {
		t.Right = insert(t.Right, v)
	}
	return t
}

func build(vals ...int) *Tree {
	var t *Tree
	for _, v := range vals {
		t = insert(t, v)
	}
	return t
}

func walk(t *Tree, ch chan int) {
	if t == nil {
		return
	}
	walk(t.Left, ch)
	ch <- t.Value
	walk(t.Right, ch)
}

func Walk(t *Tree, ch chan int) {
	walk(t, ch)
	close(ch)
}

func Same(t1, t2 *Tree) bool {
	c1, c2 := make(chan int), make(chan int)
	go Walk(t1, c1)
	go Walk(t2, c2)
	for v1 := range c1 {
		v2, ok := <-c2
		if !ok || v1 != v2 {
			// drain so the walkers can end
			for range c1 {
			}
			for range c2 {
			}
			return false
		}
	}
	_, ok := <-c2
	return !ok
}

func main() {
	a := build(5, 3, 8, 1, 4, 7, 9, 2, 6)
	b := build(1, 2, 3, 4, 5, 6, 7, 8, 9)
	c := build(5, 3, 8, 1, 4, 7, 9, 2, 10)
	d := build(1, 2, 3)
	fmt.Println(Same(a, b), Same(a, c), Same(a, d), Same(d, a), Same(nil, nil))
	ch := make(chan int)
	go Walk(a, ch)
	for v := range ch {
		fmt.Print(v, ",")
	}
	fmt.Println()
}

// Output:
// true false false false true
// 1,2,3,4,5,6,7,8,9,
