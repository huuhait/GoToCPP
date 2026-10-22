package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	var m sync.Map
	for i := 0; i < 1000; i++ {
		m.Store(i, i)
	}
	var wg sync.WaitGroup
	start := time.Now()
	for g := 0; g < 64; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				switch i % 4 {
				case 0:
					m.Store((g*31+i)%1500, i)
				case 1:
					m.Load((g + i) % 1500)
				case 2:
					n := 0
					m.Range(func(k, v any) bool {
						n++
						return n < 3
					})
				case 3:
					m.Delete((g*7 + i) % 1500)
				}
			}
		}()
	}
	wg.Wait()
	fmt.Println("elapsed ms:", time.Since(start).Milliseconds())
}
