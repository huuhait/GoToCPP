// phase: 08

// A goroutine's stack is big enough for a deep recursion (8 MiB).
package main

import "fmt"

func depth(n int) int {
	if n == 0 {
		return 0
	}
	var pad [16]int
	pad[n%16] = n
	return depth(n-1) + 1 + pad[(n+1)%16]*0
}

func main() {
	out := make(chan int)
	go func() { out <- depth(20000) }()
	fmt.Println(<-out)
	go func() { out <- depth(1000) }()
	fmt.Println(<-out)
}

// Output:
// 20000
// 1000
