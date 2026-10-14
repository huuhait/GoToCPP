// phase: 03

// Recursive types: a linked list and a binary tree through pointers, a tree whose children are
// a slice of the node type, and types that refer to each other. An assignment whose value is
// reached from its target (n = n.kids[0]) copies the value before it frees the old one.
package main

import "fmt"

type List struct {
	val  int
	next *List
}

type Tree struct {
	left, right *Tree
	key         int
}

type Dir struct {
	name  string
	files []File
	dirs  []Dir
}

type File struct {
	name   string
	parent *Dir
}

func insert(t *Tree, key int) *Tree {
	if t == nil {
		return &Tree{key: key}
	}
	if key < t.key {
		t.left = insert(t.left, key)
	} else {
		t.right = insert(t.right, key)
	}
	return t
}

func walk(t *Tree) {
	if t == nil {
		return
	}
	walk(t.left)
	fmt.Print(" ", t.key)
	walk(t.right)
}

func count(d Dir) int {
	n := len(d.files)
	for _, sub := range d.dirs {
		n += count(sub)
	}
	return n
}

func main() {
	var l *List
	for i := range 5 {
		l = &List{val: i, next: l}
	}
	for p := l; p != nil; p = p.next {
		fmt.Print(" ", p.val)
	}
	fmt.Println()

	var t *Tree
	for _, k := range []int{5, 3, 8, 1, 4, 9} {
		t = insert(t, k)
	}
	walk(t)
	fmt.Println()

	root := Dir{name: "/"}
	root.dirs = append(root.dirs, Dir{name: "a", files: []File{{name: "x"}, {name: "y"}}})
	root.dirs = append(root.dirs, Dir{name: "b", dirs: []Dir{{name: "c", files: []File{{name: "z"}}}}})
	// (A parent pointer to root itself would be a cycle, which reference counting never frees.)
	up := &Dir{name: "/"}
	root.files = append(root.files, File{name: "top", parent: up})
	fmt.Println(count(root), root.files[0].parent.name)

	// The value is in an array that only the target owns.
	d := root.dirs[1]
	d = d.dirs[0]
	fmt.Println(d.name, len(d.files), d.files[0].name)
	n := *l
	n = *n.next
	fmt.Println(n.val, n.next.val)
}

// Output:
//  4 3 2 1 0
//  1 3 4 5 8 9
// 4 /
// c 1 z
// 3 2
