// The twin of race_macro_unlocked.gp: the macro is a function that returns nil, called first in Put.

package main

import (
	"fmt"
	"sync"
)

func Locked(mu *sync.Mutex) func() {
	return nil
}

type Store struct {
	mu sync.Mutex
	n  int
}

func (s *Store) Put(v int) {
	if f := Locked(&s.mu); f != nil {
		defer f()
	}
	s.n += v
}

func main() {
	s := &Store{}
	var wg sync.WaitGroup
	wg.Add(2) // both Adds first: wg.Go would make the first goroutine's Done an edge to the second one's Add
	for range 2 {
		go func() {
			defer wg.Done()
			for i := range 100 {
				s.Put(i)
			}
		}()
	}
	wg.Wait()
	fmt.Println("done")
}
