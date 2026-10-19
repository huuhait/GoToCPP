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
}

func (s *Store) Total() (total int) {
	if f := Locked(&s.mu); f != nil {
		defer f()
	}
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
