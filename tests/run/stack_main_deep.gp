// phase: 03

// The runtime raises the main thread's RLIMIT_STACK to 1 GiB (Go's limit) when the program starts,
// so a pointer-recursive function of half a million frames works on the main goroutine, as in Go.
// With the system's 8 MiB (ulimit -s) it ended at about 105,000 frames of length (80 bytes each at
// -O0) with "fatal error: stack overflow". (Without ASLR the stack stops at about 127 MiB: still
// 1.6 million frames.) Goroutines keep 8 MiB (D24).
package main

import "fmt"

type node struct {
	next *node
	v    int
}

func length(n *node) int {
	if n == nil {
		return 0
	}
	return 1 + length(n.next)
}

func sum(n *node) int {
	if n == nil {
		return 0
	}
	return n.v + sum(n.next)
}

// depth recurses with a frame of a few hundred bytes: 100,000 of them are about 30 MB at -O0.
func depth(n int) int {
	var pad [32]int
	pad[n%32] = n
	if n == 0 {
		return pad[0]
	}
	return depth(n-1) + pad[n%32] - n + 1
}

func main() {
	var head *node
	for i := 0; i < 500000; i++ {
		head = &node{next: head, v: i % 7}
	}
	fmt.Println(length(head), sum(head))
	fmt.Println(depth(100000))
}

// Output:
// 500000 1499994
// 100000
