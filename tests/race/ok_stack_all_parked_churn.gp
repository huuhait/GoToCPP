// phase: 08

// runtime.Stack(buf, true) asks every parked or sleeping goroutine for its stack (it posts a
// request to the goroutine's own wait) while those goroutines end: the request and the record that
// the goroutine frees afterwards are runtime bookkeeping, so no race is reported (real Go reports
// none: it stops the world). Three goroutines dump all stacks at the same time while main starts
// 2000 goroutines that sleep, then wait on a channel, and end.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
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
		gate := make(chan bool)
		for g := 0; g < 50; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(50 * time.Microsecond)
				<-gate
			}()
		}
		runtime.Gosched() // (the goroutines end while the dumps reach them: a sleep here made the
		// window too narrow for a mutant without the edge to fail in one run)
		close(gate)
		wg.Wait()
	}
	close(stop)
	dumpers.Wait()
	fmt.Println("done", counts[0] > 0, counts[1] > 0, counts[2] > 0)
}

// Output:
// done true true true
