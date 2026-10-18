// phase: 08

// Goroutines that start goroutines: a binary tree of them adds up the leaves.
package main

import "fmt"

func sumTree(depth int, base int, out chan<- int) {
	if depth == 0 {
		out <- base
		return
	}
	sub := make(chan int, 2)
	go sumTree(depth-1, base*2, sub)
	go sumTree(depth-1, base*2+1, sub)
	out <- <-sub + <-sub
}

func main() {
	for d := 0; d <= 6; d++ {
		out := make(chan int)
		go sumTree(d, 1, out)
		fmt.Println(d, <-out)
	}
}

// Output:
// 0 1
// 1 5
// 2 22
// 3 92
// 4 376
// 5 1520
// 6 6112
