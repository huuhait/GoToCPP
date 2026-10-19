// phase: 09

// Stress: deep recursion on the main goroutine and on other goroutines (GOCPP_STRESS=1: 50000 frames;
// default: 10000). Real Go grows the stack up to 1 GB; gocpp's stacks are 8 MiB, which does not hold 100000 frames of this
// kind (D7, D24), so the big size stays below that.
// Run it big with: GOCPP_STRESS=1 go test ./tests -run 'Run/stress_recursion'
package main

import (
	"fmt"
	"os"
	"sync"
)

func scale(small, big int) int {
	if os.Getenv("GOCPP_STRESS") == "1" {
		return big
	}
	return small
}

func sum(n int) int {
	if n == 0 {
		return 0
	}
	return n + sum(n-1)
}

type node struct{ next *node }

func length(n *node) int {
	if n == nil {
		return 0
	}
	return 1 + length(n.next)
}

func ackermann(m, n int) int {
	switch {
	case m == 0:
		return n + 1
	case n == 0:
		return ackermann(m-1, 1)
	}
	return ackermann(m-1, ackermann(m, n-1))
}

func main() {
	depth := scale(10000, 50000)
	fmt.Println("main:", sum(depth) == depth*(depth+1)/2)

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sum(depth) == depth*(depth+1)/2 {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println("goroutines:", ok)

	var head *node
	for i := 0; i < depth; i++ {
		head = &node{head}
	}
	fmt.Println("list:", length(head) == depth)
	head = nil

	fmt.Println("ackermann(2, 300):", ackermann(2, 300))
	fmt.Println("ackermann(3, 6):", ackermann(3, 6))
}

// Output:
// main: true
// goroutines: 8
// list: true
// ackermann(2, 300): 603
// ackermann(3, 6): 509
