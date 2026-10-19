// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference
// Stderr: (?m)^goroutine 1 \[running\]:

// A nil pointer to an instance: field access panics with Go's nil dereference message and exit code 2.
package main

import "fmt"

type Node[T any] struct {
	Val  T
	Next *Node[T]
}

func (n *Node[T]) Last() T {
	for n.Next != nil {
		n = n.Next
	}
	return n.Val
}

func main() {
	n := &Node[int]{1, &Node[int]{2, nil}}
	fmt.Println(n.Last())
	var m *Node[int]
	fmt.Println(m.Last())
}

// Output:
// 2
