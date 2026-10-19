// phase: 08

// Compare-and-swap loops: a running maximum, a lock-free stack of indexes, and a spin lock built
// on CompareAndSwap protecting a plain counter.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type spinLock struct{ v atomic.Int32 }

func (s *spinLock) Lock() {
	for !s.v.CompareAndSwap(0, 1) {
	}
}
func (s *spinLock) Unlock() { s.v.Store(0) }

func main() {
	var max atomic.Int64
	var wg sync.WaitGroup
	for g := 1; g <= 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				v := int64(g*1000 + i)
				for {
					cur := max.Load()
					if v <= cur || max.CompareAndSwap(cur, v) {
						break
					}
				}
			}
		}()
	}
	wg.Wait()
	fmt.Println("max:", max.Load())

	// A Treiber stack of indexes into a slice.
	const n = 1000
	next := make([]int32, n)
	var head atomic.Int32
	head.Store(-1)
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := g; i < n; i += 10 {
				for {
					h := head.Load()
					next[i] = h
					if head.CompareAndSwap(h, int32(i)) {
						break
					}
				}
			}
		}()
	}
	wg.Wait()
	count, sum := 0, 0
	for h := head.Load(); h != -1; h = next[h] {
		count++
		sum += int(h)
	}
	fmt.Println("stack:", count, sum)

	var sl spinLock
	counter := 0
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5000; i++ {
				sl.Lock()
				counter++
				sl.Unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println("spin lock counter:", counter)
}

// Output:
// max: 20999
// stack: 1000 499500
// spin lock counter: 40000
