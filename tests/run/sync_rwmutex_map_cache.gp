// phase: 08

// A cache guarded by an RWMutex: readers look up, writers fill in; the final contents are exact.
package main

import (
	"fmt"
	"sync"
)

type Cache struct {
	mu   sync.RWMutex
	data map[int]int
	hits int
}

func (c *Cache) Get(k int) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[k]
	return v, ok
}

func (c *Cache) Put(k, v int) {
	c.mu.Lock()
	c.data[k] = v
	c.mu.Unlock()
}

func main() {
	c := &Cache{data: map[int]int{}}
	var wg sync.WaitGroup
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := w; k < 1000; k += 10 {
				c.Put(k, k*k)
			}
		}()
	}
	var found [8]int
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 1000; k++ {
				if v, ok := c.Get(k); ok && v != k*k {
					found[r] = -1 << 30
				}
			}
		}()
	}
	wg.Wait()
	sum := 0
	for k := 0; k < 1000; k++ {
		v, ok := c.Get(k)
		if !ok {
			fmt.Println("missing", k)
		}
		sum += v
	}
	fmt.Println(len(c.data), sum, found)
}

// Output:
// 1000 332833500 [0 0 0 0 0 0 0 0]
