// phase: 08

// atomic.Bool: Load, Store, Swap and CompareAndSwap; a flag that stops a goroutine.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var b atomic.Bool
	fmt.Println(b.Load())
	b.Store(true)
	fmt.Println(b.Load(), b.Swap(false), b.Load(), b.Swap(false))
	fmt.Println(b.CompareAndSwap(true, true), b.CompareAndSwap(false, true), b.Load(), b.CompareAndSwap(false, false))

	// Only one of the goroutines wins the CompareAndSwap.
	var once atomic.Bool
	var wg sync.WaitGroup
	var wins atomic.Int32
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if once.CompareAndSwap(false, true) {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	fmt.Println("winners:", wins.Load(), once.Load())

	// A stop flag: the worker spins until main sets it.
	var stop atomic.Bool
	var spins atomic.Int64
	done := make(chan struct{})
	started := make(chan struct{})
	go func() {
		close(started)
		for !stop.Load() {
			spins.Add(1)
		}
		close(done)
	}()
	<-started
	stop.Store(true)
	<-done
	fmt.Println("stopped:", stop.Load(), spins.Load() >= 0)
}

// Output:
// false
// true true false false
// false true true false
// winners: 1 true
// stopped: true true
