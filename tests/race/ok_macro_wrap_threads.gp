// phase: 08

// Wrap macros that run the function's code on another thread, once, or under recover (step 10.13; the
// wrap spike's InThread, Once and Recover, race-clean): InThread starts #body() in a goroutine and waits for
// it (the wait is the synchronization that makes the results and the writes of the code visible to the
// caller), Once runs it one time however many goroutines call, Recover stops a panic. Four goroutines call each.
package main

import (
	"fmt"
	"sync"
)

macro InThread() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		#body()
	}()
	<-done
}

macro Once(o *sync.Once) {
	o.Do(func() { #body() })
}

macro Recover(on *bool) {
	defer func() {
		if r := recover(); r != nil {
			*on = true
		}
	}()
	#body()
}

var (
	setupOnce  sync.Once
	setupCalls int
	mu         sync.Mutex
	recovered  = map[int]bool{}
)

#Once(&setupOnce)
func setup() {
	setupCalls++
}

#InThread()
func square(n int) (r int) {
	r = n * n
	return r
}

func risky(i int) (out int) {
	var hit bool
	out = guarded(i, &hit)
	mu.Lock()
	recovered[i] = hit
	mu.Unlock()
	return out
}

#Recover(hit)
func guarded(i int, hit *bool) int {
	if i%2 == 1 {
		panic("odd")
	}
	return i * 10
}

func main() {
	var wg sync.WaitGroup
	results := make([]int, 4)
	for g := range 4 {
		wg.Go(func() {
			setup()
			results[g] = square(g+1) + risky(g)
		})
	}
	wg.Wait()
	fmt.Println(setupCalls, results, recovered)
}

// Output:
// 1 [1 4 29 16] map[0:false 1:true 2:false 3:true]
