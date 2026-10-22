// phase: 08

// sync.Map under every operation at once: Range promotes the dirty map while other goroutines
// store, load, swap, compare, delete and clear. The program has no data race.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var m sync.Map
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				key := (g*13 + i) % 40
				switch i % 8 {
				case 0:
					m.Store(key, i)
				case 1:
					m.Load(key)
				case 2:
					m.Range(func(k, v any) bool {
						return k.(int)%2 == 0
					})
				case 3:
					m.Delete(key)
				case 4:
					m.LoadOrStore(key, i)
				case 5:
					m.Swap(key, i)
				case 6:
					m.CompareAndSwap(key, i, i+1)
					m.CompareAndDelete(key, i)
				case 7:
					if i%64 == 7 {
						m.Clear()
					}
				}
			}
		}()
	}
	wg.Wait()
	fmt.Println("done")
}

// Output:
// done
