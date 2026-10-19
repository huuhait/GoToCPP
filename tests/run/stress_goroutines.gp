// phase: 09

// Stress: many goroutines alive at the same time (GOCPP_STRESS=1: 10000; default: 500). They block on one
// gate, are released together, pass a token along a chain, and a burst of short-lived goroutines follows.
// Run it big with: GOCPP_STRESS=1 go test ./tests -run 'Run/stress_goroutines'
package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

func scale(small, big int) int {
	if os.Getenv("GOCPP_STRESS") == "1" {
		return big
	}
	return small
}

func main() {
	n := scale(500, 10000)

	// n goroutines parked on one channel at the same time
	gate := make(chan struct{})
	var parked sync.WaitGroup
	var done sync.WaitGroup
	var released atomic.Int64
	parked.Add(n)
	done.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer done.Done()
			parked.Done()
			<-gate
			released.Add(int64(i))
		}()
	}
	parked.Wait()
	fmt.Println("goroutines alive:", runtime.NumGoroutine() >= n)
	close(gate)
	done.Wait()
	fmt.Println("released sum ok:", released.Load() == int64(n)*int64(n-1)/2)

	// a chain: each goroutine adds one and passes the value on (n links)
	first := make(chan int)
	in := first
	for i := 0; i < n; i++ {
		out := make(chan int)
		go func(in <-chan int, out chan<- int) { out <- 1 + <-in }(in, out)
		in = out
	}
	first <- 0
	fmt.Println("chain result:", <-in == n)

	// a burst of short-lived goroutines, many generations
	rounds := scale(20, 100)
	var total atomic.Int64
	for r := 0; r < rounds; r++ {
		var wg sync.WaitGroup
		for i := 0; i < n/10; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); total.Add(1) }()
		}
		wg.Wait()
	}
	fmt.Println("burst total ok:", total.Load() == int64(rounds)*int64(n/10))
}

// Output:
// goroutines alive: true
// released sum ok: true
// chain result: true
// burst total ok: true
