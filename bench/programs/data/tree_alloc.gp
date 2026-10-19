// bench: tree_alloc
// category: data
// size: 400000
// smoke: 2000
// twin: yes
// about: pointer-based binary search tree: recursive insert, lookup, height, in-order walk, drop
package main

import "fmt"

var seed uint64 = 99

func rnd() uint64 {
	seed = seed*6364136223846793005 + 1442695040888963407
	return seed >> 33
}

type tree struct {
	key         int
	left, right *tree
}

func insert(t *tree, k int) *tree {
	if t == nil {
		return &tree{key: k}
	}
	if k < t.key {
		t.left = insert(t.left, k)
	} else if k > t.key {
		t.right = insert(t.right, k)
	}
	return t
}

func find(t *tree, k int) bool {
	for t != nil {
		if k < t.key {
			t = t.left
		} else if k > t.key {
			t = t.right
		} else {
			return true
		}
	}
	return false
}

func height(t *tree) int {
	if t == nil {
		return 0
	}
	l, r := height(t.left), height(t.right)
	if l > r {
		return l + 1
	}
	return r + 1
}

func walk(t *tree, acc *uint64, count *int) {
	if t == nil {
		return
	}
	walk(t.left, acc, count)
	*count++
	*acc = *acc*31 + uint64(t.key)
	walk(t.right, acc, count)
}

func main() {
	n := benchN
	var root *tree
	for i := 0; i < n; i++ {
		root = insert(root, int(rnd()%uint64(4*n)))
	}
	hits := 0
	for i := 0; i < n; i++ {
		if find(root, int(rnd()%uint64(4*n))) {
			hits++
		}
	}
	var acc uint64
	count := 0
	walk(root, &acc, &count)
	fmt.Println(count, hits, height(root), acc)
	root = nil
}
