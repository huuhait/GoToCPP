// phase: 07

// Recursive traversal of a tree with generic helpers: a fold, a map into a slice, and a search.
package main

import "fmt"

type Node struct {
	Val         int
	Left, Right *Node
}

func Insert(n *Node, v int) *Node {
	if n == nil {
		return &Node{Val: v}
	}
	if v < n.Val {
		n.Left = Insert(n.Left, v)
	} else {
		n.Right = Insert(n.Right, v)
	}
	return n
}

func Fold[A any](n *Node, acc A, f func(A, int) A) A {
	if n == nil {
		return acc
	}
	acc = Fold(n.Left, acc, f)
	acc = f(acc, n.Val)
	return Fold(n.Right, acc, f)
}

func Collect[T any](n *Node, f func(int) T) []T {
	return Fold(n, []T(nil), func(out []T, v int) []T { return append(out, f(v)) })
}

func Find[T comparable](n *Node, f func(int) T, want T) bool {
	if n == nil {
		return false
	}
	return f(n.Val) == want || Find(n.Left, f, want) || Find(n.Right, f, want)
}

func main() {
	var root *Node
	for _, v := range []int{5, 3, 8, 1, 4, 9, 7} {
		root = Insert(root, v)
	}
	fmt.Println(Collect(root, func(v int) int { return v }))
	fmt.Println(Collect(root, func(v int) string { return fmt.Sprint("n", v) }))
	fmt.Println(Fold(root, 0, func(a, v int) int { return a + v }))
	fmt.Println(Fold(root, "", func(a string, v int) string { return a + fmt.Sprint(v) }))
	fmt.Println(Find(root, func(v int) int { return v * 2 }, 16), Find(root, func(v int) bool { return v > 100 }, true))
	fmt.Println(Find(root, func(v int) string { return fmt.Sprint(v) }, "4"))
}

// Output:
// [1 3 4 5 7 8 9]
// [n1 n3 n4 n5 n7 n8 n9]
// 37
// 1345789
// true false
// true
