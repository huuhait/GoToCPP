// phase: 08

// runtime.Stack(buf, true) lists the other goroutines while goroutines start and end: the
// registry that the report walks is runtime bookkeeping, not data of the program, so no race is
// reported (real Go reports none: it stops the world). Three goroutines dump all stacks at the
// same time while main starts and ends 2000 goroutines.
package main

import (
	"fmt"
	"runtime"
	"sync"
)

func main() {
	stop := make(chan bool)
	var dumpers sync.WaitGroup
	counts := make([]int, 3)
	for d := range counts {
		dumpers.Add(1)
		go func() {
			defer dumpers.Done()
			buf := make([]byte, 1<<16)
			for {
				select {
				case <-stop:
					return
				default:
				}
				if runtime.Stack(buf, true) > 0 {
					counts[d]++
				}
			}
		}()
	}
	var wg sync.WaitGroup
	for round := 0; round < 40; round++ {
		for g := 0; g < 50; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
			}()
		}
		wg.Wait()
	}
	close(stop)
	dumpers.Wait()
	fmt.Println("done", counts[0] > 0, counts[1] > 0, counts[2] > 0)
}

// Output:
// done true true true
