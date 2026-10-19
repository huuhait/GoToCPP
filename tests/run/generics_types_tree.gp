// phase: 07

// A generic binary search tree with a comparison function, and a generic walk.
package main

import "fmt"

type Tree[T any] struct {
	Left, Right *Tree[T]
	Val         T
}

func (t *Tree[T]) Insert(v T, less func(a, b T) bool) *Tree[T] {
	if t == nil {
		return &Tree[T]{Val: v}
	}
	if less(v, t.Val) {
		t.Left = t.Left.Insert(v, less)
	} else {
		t.Right = t.Right.Insert(v, less)
	}
	return t
}

func (t *Tree[T]) Walk(f func(T)) {
	if t == nil {
		return
	}
	t.Left.Walk(f)
	f(t.Val)
	t.Right.Walk(f)
}

func (t *Tree[T]) Height() int {
	if t == nil {
		return 0
	}
	return 1 + max(t.Left.Height(), t.Right.Height())
}

func (t *Tree[T]) Contains(v T, eq func(a, b T) bool) bool {
	if t == nil {
		return false
	}
	return eq(t.Val, v) || t.Left.Contains(v, eq) || t.Right.Contains(v, eq)
}

func main() {
	var ints *Tree[int]
	for _, v := range []int{5, 2, 8, 1, 9, 3} {
		ints = ints.Insert(v, func(a, b int) bool { return a < b })
	}
	var out []int
	ints.Walk(func(v int) { out = append(out, v) })
	fmt.Println(out, ints.Height(), ints.Contains(8, func(a, b int) bool { return a == b }), ints.Contains(7, func(a, b int) bool { return a == b }))
	var words *Tree[string]
	for _, w := range []string{"pear", "apple", "zebra", "mango"} {
		words = words.Insert(w, func(a, b string) bool { return a < b })
	}
	var ws []string
	words.Walk(func(s string) { ws = append(ws, s) })
	fmt.Println(ws, words.Height())
	var empty *Tree[float64]
	fmt.Println(empty.Height())
	fmt.Printf("%T\n", words)
}

// Output:
// [1 2 3 5 8 9] 3 true false
// [apple mango pear zebra] 3
// 0
// *main.Tree[string]
