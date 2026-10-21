package main

import (
	"fmt"
	"sync"
)

func Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

type Store struct {
	mu sync.Mutex
	m  map[string]int
}

func (s *Store) Put(k string, v int) {
	if f := Locked(&s.mu); f != nil {
		defer f()
	}
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
