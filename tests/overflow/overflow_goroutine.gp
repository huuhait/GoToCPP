// phase: 08
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$
// Stderr: (?m)^goroutine \d+ .*\[running\]:$

// Endless recursion in a goroutine kills the whole program the same way, while main waits.
package main

import (
	"fmt"
	"sync"
)

var sink [4]int

func recurse(n int) int {
	sink[n%4] = n
	return recurse(n+1) + sink[(n+1)%4]
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { recover() }()
		fmt.Println(recurse(0))
	}()
	wg.Wait()
	fmt.Println("not reached")
}

// Output:
