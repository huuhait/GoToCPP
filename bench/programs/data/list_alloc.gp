// bench: list_alloc
// category: data
// size: 12000
// smoke: 20
// twin: yes
// about: singly linked lists of heap nodes: allocate, traverse, reverse, drop (reference counting cost)
package main

import "fmt"

type node struct {
	val  int
	next *node
}

func build(n, base int) *node {
	var head *node
	for i := 0; i < n; i++ {
		head = &node{val: base + i, next: head}
	}
	return head
}

func sum(l *node) uint64 {
	var s uint64
	for p := l; p != nil; p = p.next {
		s += uint64(p.val)
	}
	return s
}

func reverse(l *node) *node {
	var prev *node
	for l != nil {
		next := l.next
		l.next = prev
		prev = l
		l = next
	}
	return prev
}

func main() {
	var total uint64
	for r := 0; r < benchN; r++ {
		l := build(1000, r)
		total += sum(l)
		l = reverse(l)
		total += uint64(l.val) * 3
		total += sum(l)
		l = nil
	}
	fmt.Println(total)
}
