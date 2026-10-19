// phase: 08

// A WaitGroup as a struct field, used through the pointer to the struct, and copied (empty)
// before first use.
package main

import (
	"fmt"
	"sync"
)

type Pool struct {
	wg      sync.WaitGroup
	mu      sync.Mutex
	results []int
}

func (p *Pool) Run(n int, f func(int) int) {
	for i := 0; i < n; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			r := f(i)
			p.mu.Lock()
			p.results = append(p.results, r)
			p.mu.Unlock()
		}()
	}
	p.wg.Wait()
}

func sum(s []int) int {
	t := 0
	for _, v := range s {
		t += v
	}
	return t
}

func main() {
	var proto Pool
	a, b := proto, proto
	a.Run(10, func(i int) int { return i })
	b.Run(5, func(i int) int { return i * 10 })
	fmt.Println(len(a.results), sum(a.results), len(b.results), sum(b.results))
	a.results = nil
	a.Run(3, func(i int) int { return 1 })
	fmt.Println(len(a.results))
}

// Output:
// 10 45 5 100
// 3
