// phase: 07

// Unexported generic types, fields and methods; lower-case names in the instance's text.
package main

import "fmt"

type node[T any] struct {
	val  T
	next *node[T]
}

type queue[T any] struct {
	head, tail *node[T]
	n          int
}

func (q *queue[T]) push(v T) {
	n := &node[T]{val: v}
	if q.tail == nil {
		q.head, q.tail = n, n
	} else {
		q.tail.next = n
		q.tail = n
	}
	q.n++
}

func (q *queue[T]) pop() (v T, ok bool) {
	if q.head == nil {
		return v, false
	}
	v = q.head.val
	q.head = q.head.next
	if q.head == nil {
		q.tail = nil
	}
	q.n--
	return v, true
}

func main() {
	var q queue[string]
	q.push("a")
	q.push("b")
	v, _ := q.pop()
	fmt.Println(v, q.n)
	var r queue[int]
	_, ok := r.pop()
	fmt.Println(ok)
	fmt.Printf("%T %T %v\n", q, q.head, q.n)
}

// Output:
// a 1
// false
// main.queue[string] *main.node[string] 1
