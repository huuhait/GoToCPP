// phase: 08

// Equivalent binary trees (the Go tour exercise): walk two trees in goroutines and compare the
// values they send.
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

func build(vals []int) *Tree {
	var t *Tree
	for _, v := range vals {
		t = insert(t, v)
	}
	return t
}

func walkTree(t *Tree, ch chan<- int) {
	if t == nil {
		return
	}
	walkTree(t.Left, ch)
	ch <- t.Value
	walkTree(t.Right, ch)
}

func walk(t *Tree, ch chan<- int) {
	walkTree(t, ch)
	close(ch)
}

func same(a, b *Tree) bool {
	c1, c2 := make(chan int), make(chan int)
	go walk(a, c1)
	go walk(b, c2)
	for {
		v1, ok1 := <-c1
		v2, ok2 := <-c2
		if ok1 != ok2 || v1 != v2 {
			return false
		}
		if !ok1 {
			return true
		}
	}
}

func main() {
	a := build([]int{5, 3, 8, 1, 4, 7, 9, 2, 6})
	b := build([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
	c := build([]int{1, 2, 3, 4, 5, 6, 7, 8, 10})
	fmt.Println(same(a, b), same(a, c), same(b, b))
}

// Output:
// true false true
