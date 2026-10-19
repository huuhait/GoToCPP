// phase: 08
// lsan: cycle

// A counting semaphore from a Mutex and a Cond limits how many goroutines run at once.
//
// (The Cond's Locker is a pointer into the struct that holds the Cond: a reference cycle, which
// reference counting never frees, divergence D1. LeakSanitizer is off for this test.)
package main

import (
	"fmt"
	"sync"
)

type Semaphore struct {
	mu    sync.Mutex
	cond  *sync.Cond
	avail int
}

func NewSemaphore(n int) *Semaphore {
	s := &Semaphore{avail: n}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func (s *Semaphore) Acquire() {
	s.mu.Lock()
	for s.avail == 0 {
		s.cond.Wait()
	}
	s.avail--
	s.mu.Unlock()
}

func (s *Semaphore) Release() {
	s.mu.Lock()
	s.avail++
	s.mu.Unlock()
	s.cond.Signal()
}

func main() {
	sem := NewSemaphore(3)
	var mu sync.Mutex
	running, maxRunning, total := 0, 0, 0
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem.Acquire()
			defer sem.Release()
			mu.Lock()
			running++
			if running > maxRunning {
				maxRunning = running
			}
			mu.Unlock()
			s := 0
			for k := 0; k < 10000; k++ {
				s += k
			}
			mu.Lock()
			running--
			total += s
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Println("max running <= 3:", maxRunning <= 3, "total:", total, "running:", running)
}

// Output:
// max running <= 3: true total: 1499850000 running: 0
