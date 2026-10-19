// phase: 08

// A start-style macro that locks (step 10.13): #Locked(&s.mu) on top of a method is the first statement of the
// method and its unlock is deferred, so 16 goroutines that call Put and Total at the same time share the map
// without a race (the spike's Locked, race-clean). The same program with a macro that locks nothing is
// race_macro_unlocked.gp.
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
}

#Locked(&s.mu)
func (s *Store) Total() (total int) {
	for _, v := range s.m {
		total += v
	}
	return total
}

func main() {
	s := &Store{m: map[string]int{}}
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Go(func() {
			for i := range 100 {
				s.Put(fmt.Sprint("k", (g+i)%5), 1)
				_ = s.Total()
			}
		})
	}
	wg.Wait()
	fmt.Println(s.Total())
}

// Output:
// 1600
