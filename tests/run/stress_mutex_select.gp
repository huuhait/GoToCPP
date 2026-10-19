// phase: 09

// Stress: locks and select under contention (GOCPP_STRESS=1: 64 goroutines x 20000 rounds; default: 16 x 500).
// Run it big with: GOCPP_STRESS=1 go test ./tests -run 'Run/stress_mutex_select'
package main

import (
	"fmt"
	"os"
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
	workers := scale(16, 64)
	rounds := scale(500, 20000)

	var mu sync.Mutex
	var rw sync.RWMutex
	var wg sync.WaitGroup
	counter, shared := 0, 0
	var reads atomic.Int64
	for g := 0; g < workers; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				mu.Lock()
				counter++
				mu.Unlock()
				if i%8 == 0 {
					rw.Lock()
					shared++
					rw.Unlock()
				} else {
					rw.RLock()
					_ = shared
					reads.Add(1)
					rw.RUnlock()
				}
			}
		}()
	}
	wg.Wait()
	writes := (rounds + 7) / 8
	fmt.Println(counter == workers*rounds, shared == workers*writes, reads.Load() == int64(workers*(rounds-writes)))

	// select: all workers send on two channels, one receiver takes both with a quit channel
	a, b := make(chan int), make(chan int)
	quit := make(chan struct{})
	var senders sync.WaitGroup
	for g := 0; g < workers; g++ {
		senders.Add(1)
		go func() {
			defer senders.Done()
			for i := 0; i < rounds; i++ {
				select {
				case a <- 1:
				case b <- 2:
				}
			}
		}()
	}
	go func() { senders.Wait(); close(quit) }()
	total, count := 0, 0
loop:
	for {
		select {
		case v := <-a:
			total += v
			count++
		case v := <-b:
			total += v
			count++
		case <-quit:
			break loop
		}
	}
	fmt.Println(count == workers*rounds, total >= count && total <= 2*count)
}

// Output:
// true true true
// true true
