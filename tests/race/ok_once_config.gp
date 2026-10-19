// phase: 08

// Many goroutines call Once.Do and then read what its function wrote: the function's work happens
// before every Do that returns after it (no mutex of the program, only the Once and a WaitGroup).
package main

import (
	"fmt"
	"sync"
)

var (
	once sync.Once
	cfg  map[string]int
	name string
)

func load() {
	cfg = map[string]int{"a": 1, "b": 2, "c": 3}
	name = "config"
}

func main() {
	var wg sync.WaitGroup
	sums := make([]int, 8)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				once.Do(load)
				sums[g] += cfg["a"] + cfg["b"] + cfg["c"] + len(name)
			}
		}()
	}
	wg.Wait()
	total := 0
	for _, s := range sums {
		total += s
	}
	fmt.Println(total)
}

// Output:
// 1920
