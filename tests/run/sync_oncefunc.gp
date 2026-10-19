// phase: 08

// OnceFunc returns a function that calls f once; if f panics, every call panics with the same value.
package main

import (
	"fmt"
	"sync"
)

func main() {
	calls := 0
	f := sync.OnceFunc(func() { calls++; fmt.Println("init") })
	f()
	f()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); f() }()
	}
	wg.Wait()
	fmt.Println("calls:", calls)

	g := sync.OnceFunc(func() { panic("bad init") })
	for i := 0; i < 3; i++ {
		func() {
			defer func() { fmt.Println(i, "recovered:", recover()) }()
			g()
		}()
	}
}

// Output:
// init
// calls: 1
// 0 recovered: bad init
// 1 recovered: bad init
// 2 recovered: bad init
