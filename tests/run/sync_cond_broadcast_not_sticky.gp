// phase: 08
// lsan: cycle

// Signal and Broadcast wake only the goroutines that are waiting at that moment: with nobody waiting they do
// nothing, and a Wait that starts after a Broadcast stays blocked until the next Signal or Broadcast.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

type gate struct {
	mu      sync.Mutex
	cond    *sync.Cond
	waiting int // goroutines that have started Wait (counted under mu, which Wait releases)
	woken   int
}

func newGate() *gate {
	g := &gate{}
	g.cond = sync.NewCond(&g.mu)
	return g
}

// waiter blocks in Wait and counts itself woken afterwards.
func (g *gate) waiter(wg *sync.WaitGroup) {
	defer wg.Done()
	g.mu.Lock()
	g.waiting++
	g.cond.Wait()
	g.woken++
	g.mu.Unlock()
}

// until polls (bounded) for a condition under the lock; taking the lock proves that the waiters counted so far
// have released it inside Wait.
func (g *gate) until(f func() bool) bool {
	for i := 0; i < 20000; i++ {
		g.mu.Lock()
		ok := f()
		g.mu.Unlock()
		if ok {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// quiet gives a wrongly woken goroutine every chance to run, then reports how many are woken.
func (g *gate) quiet() int {
	for i := 0; i < 50; i++ {
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.woken
}

func main() {
	g := newGate()
	var wg sync.WaitGroup

	// nobody waits: nothing is stored (the Signal alone: a Broadcast after it would reset the counters)
	g.cond.Signal()
	wg.Add(1)
	go g.waiter(&wg)
	g.until(func() bool { return g.waiting == 1 })
	fmt.Println("after an early Signal, the waiter sleeps:", g.quiet() == 0)

	// Broadcast wakes the one that waits
	g.cond.Broadcast()
	wg.Wait()
	fmt.Println("woken by Broadcast:", g.woken)

	// a Wait that starts after the Broadcast is not woken by it
	wg.Add(1)
	go g.waiter(&wg)
	g.until(func() bool { return g.waiting == 2 })
	fmt.Println("late waiter stays blocked:", g.quiet() == 1)

	// Signal wakes exactly one of two
	wg.Add(1)
	go g.waiter(&wg)
	g.until(func() bool { return g.waiting == 3 })
	g.cond.Signal()
	g.until(func() bool { return g.woken == 2 })
	fmt.Println("one woken by Signal, the other still blocked:", g.quiet() == 2)

	g.cond.Broadcast()
	wg.Wait()
	fmt.Println("all woken:", g.woken)
}

// Output:
// after an early Signal, the waiter sleeps: true
// woken by Broadcast: 1
// late waiter stays blocked: true
// one woken by Signal, the other still blocked: true
// all woken: 3
