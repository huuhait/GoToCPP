// phase: 08

// A struct that embeds *sync.RWMutex (a pointer) and one that embeds sync.RWMutex by value; the
// promoted methods and the Locker interface work for both.
package main

import (
	"fmt"
	"sync"
)

type ByValue struct {
	sync.RWMutex
	data []int
}

type ByPointer struct {
	*sync.RWMutex
	n int
}

func sum(l sync.Locker, f func()) {
	l.Lock()
	defer l.Unlock()
	f()
}

func main() {
	v := &ByValue{}
	p := ByPointer{RWMutex: new(sync.RWMutex)}
	q := p // shares the mutex
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v.Lock()
			v.data = append(v.data, i)
			v.Unlock()
			sum(&p, func() { p.n++ })
			q.RLock()
			_ = q.n
			q.RUnlock()
		}()
	}
	wg.Wait()
	fmt.Println(len(v.data), p.n, q.n)
	fmt.Println(v.TryLock(), p.TryRLock(), q.TryLock())
}

// Output:
// 20 20 0
// true true false
