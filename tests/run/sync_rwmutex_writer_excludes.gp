// phase: 08

// A writer excludes readers and other writers: the value a reader sees is always a pair that a
// writer wrote completely, and no update is lost.
package main

import (
	"fmt"
	"sync"
)

type pair struct{ a, b int }

func main() {
	var rw sync.RWMutex
	var p pair
	var wg sync.WaitGroup
	bad := 0
	var badMu sync.Mutex
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				rw.Lock()
				p.a++
				p.b += 2
				rw.Unlock()
			}
		}()
	}
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				rw.RLock()
				if p.b != 2*p.a {
					badMu.Lock()
					bad++
					badMu.Unlock()
				}
				rw.RUnlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println(p.a, p.b, "torn reads:", bad)
}

// Output:
// 8000 16000 torn reads: 0
