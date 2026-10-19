// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_macro_unlocked\.g[op]:(2[0-9])\b

// A start-style macro that forgets to lock (step 10.13): ok_macro_locked.gp with the Lock and the Unlock left
// out. The macro expands to real code, so the detector sees the unprotected write of Put at its .gp line.
package main

import (
	"fmt"
	"sync"
)

macro Locked(mu *sync.Mutex) func() {
	return nil
}

type Store struct {
	mu sync.Mutex
	n  int
}

#Locked(&s.mu)
func (s *Store) Put(v int) {
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

// Output:
// done
