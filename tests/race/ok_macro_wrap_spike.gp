// phase: 08

// The wrap-macro spike (spikes/2026-10-13/macros-v3-wrap: Trace, VM, Recover, InThread, Once) under the
// race detector with 16 goroutines (exit criterion of Phase 10: "wrap macros behave as in the spike,
// race-clean"). Each goroutine calls every wrapped function; the macros count what they do with
// atomics instead of printing, so that the output does not depend on the order of the goroutines. The
// counts are what the spike's single-threaded run implies: Trace and VM see every call and leave
// every call (also the one that panics), Recover stops 6 of the 16 divisions by zero, InThread runs
// the code on another thread and its result is visible to the caller after the wait, and Once runs
// the code of setup one time. The twin is testdata/twin/ok_macro_wrap_spike.go.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var entered, left, inVM, vmCalls, recovered atomic.Int64

macro Trace() func() {
	entered.Add(1)
	return func() { left.Add(1) }
}

macro VM() {
	inVM.Add(1)
	defer inVM.Add(-1)
	vmCalls.Add(1)
	#body()
}

macro Recover() {
	defer func() {
		if recover() != nil {
			recovered.Add(1)
		}
	}()
	#body()
}

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

#Trace()
#VM()
func secret(key string) (int, bool) {
	if key == "" {
		return 0, false
	}
	return len(key) * 42, true
}

#Recover()
func risky(n int) int { return 10 / n }

#InThread()
func work(id int) string { return fmt.Sprint("job ", id, " done") }

var (
	setupOnce sync.Once
	setups    int
)

#Once(&setupOnce)
func setup() int {
	setups++
	return setups
}

func main() {
	const goroutines = 16
	var wg sync.WaitGroup
	secrets := make([]int, goroutines)
	divided := make([]int, goroutines)
	jobs := make([]string, goroutines)
	for g := range goroutines {
		wg.Go(func() {
			key := "key"
			if g%4 == 0 {
				key = ""
			}
			secrets[g], _ = secret(key)
			divided[g] = risky(g % 3)
			jobs[g] = work(g)
			setup()
		})
	}
	wg.Wait()

	sum := 0
	for _, n := range secrets {
		sum += n
	}
	fmt.Println("secrets", sum)
	fmt.Println("divided", divided)
	fmt.Println(jobs[0], "/", jobs[15])
	fmt.Println("setups", setups, "recovered", recovered.Load())
	fmt.Println("trace", entered.Load(), left.Load(), "vm", inVM.Load(), vmCalls.Load())
}

// Output:
// secrets 1512
// divided [0 10 5 0 10 5 0 10 5 0 10 5 0 10 5 0]
// job 0 done / job 15 done
// setups 1 recovered 6
// trace 16 16 vm 0 16
