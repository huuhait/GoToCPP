// phase: 08

// A struct that embeds sync.Mutex has Lock and Unlock promoted; another one keeps the mutex in a
// field. Both protect their counters from 20 goroutines.
package main

import (
	"fmt"
	"sync"
)

type Counter struct {
	sync.Mutex
	n int
}

type Stats struct {
	mu   sync.Mutex
	hits map[string]int
}

func (s *Stats) Hit(k string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits[k]++
}

func main() {
	var c Counter
	s := &Stats{hits: map[string]int{}}
	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				c.Lock()
				c.n++
				c.Unlock()
				s.Hit([]string{"a", "b", "c"}[(g+i)%3])
			}
		}()
	}
	wg.Wait()
	fmt.Println(c.n, s.hits["a"]+s.hits["b"]+s.hits["c"], len(s.hits))
	// The promoted methods are in the method set, so the struct is a sync.Locker through a pointer.
	var l sync.Locker = &c
	l.Lock()
	fmt.Println(c.TryLock())
	l.Unlock()
	fmt.Println(c.TryLock())
}

// Output:
// 10000 10000 3
// false
// true
