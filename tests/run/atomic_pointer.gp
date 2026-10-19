// phase: 08
// pending: generic types (step 7.3): atomic.Pointer[T] is a generic type

// atomic.Pointer[T]: Load, Store, Swap and CompareAndSwap of a *T, with readers racing a writer.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type config struct {
	name string
	n    int
}

func main() {
	var p atomic.Pointer[config]
	fmt.Println(p.Load() == nil)
	a, b := &config{"a", 1}, &config{"b", 2}
	p.Store(a)
	fmt.Println(p.Load().name, p.CompareAndSwap(b, b), p.CompareAndSwap(a, b), p.Load().name)
	fmt.Println(p.Swap(a).name, p.Load().name)

	var cur atomic.Pointer[config]
	cur.Store(&config{"v0", 0})
	var wg sync.WaitGroup
	torn := 0
	var mu sync.Mutex
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				c := cur.Load()
				if c.name != fmt.Sprint("v", c.n) {
					mu.Lock()
					torn++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 2000; i++ {
			cur.Store(&config{fmt.Sprint("v", i), i})
		}
	}()
	wg.Wait()
	fmt.Println("torn:", torn, cur.Load().n)
}

// Output:
// true
// a false true b
// b a
// torn: 0 2000
