// phase: 08

// If f panics, Do considers it done: later calls return without calling f.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var once sync.Once
	calls := 0
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		once.Do(func() {
			calls++
			panic("first call fails")
		})
	}()
	once.Do(func() { calls++ })
	once.Do(func() { calls++ })
	fmt.Println("calls:", calls)

	// The mutex inside the Once was released by the panic: Do from other goroutines doesn't hang.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			once.Do(func() { calls++ })
		}()
	}
	wg.Wait()
	fmt.Println("calls:", calls)
}

// Output:
// recovered: first call fails
// calls: 1
// calls: 1
