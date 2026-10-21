// phase: 08

// A start-style macro that locks, on a method that panics (exit criterion of Phase 10): the unlock is
// the macro's exit part, so it runs when Put panics, and the other goroutines do not wait for ever.
// 16 goroutines call Put 100 times each, every seventh value panics after it changed the map, and each
// goroutine recovers the panic at the call. The detector stays silent (the mutex covers the map,
// also on the way out of a panic), and every value was added exactly once. The twin is
// testdata/twin/ok_macro_locked_panic.go.
package main

import (
	"fmt"
	"sync"
)

// Locked holds mu while the function runs.
macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

type Store struct {
	mu sync.Mutex
	m  map[string]int
}

#Locked(&s.mu)
func (s *Store) Put(k string, v int) {
	s.m[k] += v
	if v%7 == 0 {
		panic("seven")
	}
}

// tryPut calls Put and reports whether it panicked.
func (s *Store) tryPut(k string, v int) (panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	s.Put(k, v)
	return false
}

func main() {
	s := &Store{m: map[string]int{}}
	var wg sync.WaitGroup
	panics := make([]int, 16)
	for g := range 16 {
		wg.Go(func() {
			for v := 1; v <= 100; v++ {
				if s.tryPut(fmt.Sprint("k", g%4), v) {
					panics[g]++
				}
			}
		})
	}
	wg.Wait()

	total, count := 0, 0
	for _, n := range s.m {
		total += n
	}
	for _, n := range panics {
		count += n
	}
	fmt.Println("total", total, "panics", count)
}

// Output:
// total 80800 panics 224
