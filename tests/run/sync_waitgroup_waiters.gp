// phase: 08

// Several goroutines Wait on one WaitGroup; all of them are released when the counter hits zero,
// and Wait on a zero counter returns at once.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var wg sync.WaitGroup
	wg.Wait()
	fmt.Println("zero counter: Wait returned")

	wg.Add(1)
	var released atomic.Int32
	var waiters sync.WaitGroup
	for i := 0; i < 6; i++ {
		waiters.Add(1)
		go func() {
			defer waiters.Done()
			wg.Wait()
			released.Add(1)
		}()
	}
	fmt.Println("released before Done:", released.Load())
	wg.Done()
	waiters.Wait()
	fmt.Println("released after Done:", released.Load())
}

// Output:
// zero counter: Wait returned
// released before Done: 0
// released after Done: 6
