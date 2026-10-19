// phase: 08

// Do guarantees that when it returns, f has finished, also for the calls that did not run f:
// they wait for the call that does. One goroutine enters Do and blocks inside f; eight others
// call Do and, when it returns, check that f finished; main waits (busy) until they have all had
// time to queue on the Once, then lets f finish.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var sink int

func spin() {
	for i := 0; i < 100000000; i++ {
		sink += i
	}
}

func main() {
	var once sync.Once
	var fdone atomic.Int32
	release := make(chan bool)
	entered := make(chan bool)
	var wg sync.WaitGroup
	bad := atomic.Int32{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		once.Do(func() {
			entered <- true
			<-release
			fdone.Store(1)
		})
	}()
	<-entered
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			once.Do(func() { bad.Add(100) }) // never runs: the first call is in progress
			if fdone.Load() != 1 {
				bad.Add(1) // Do returned before f finished
			}
		}()
	}
	spin()
	release <- true
	wg.Wait()
	fmt.Println("bad", bad.Load())
}

// Output:
// bad 0
