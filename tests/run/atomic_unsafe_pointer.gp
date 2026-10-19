// phase: 08

// LoadPointer, StorePointer, SwapPointer and CompareAndSwapPointer on an unsafe.Pointer.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

type config struct {
	name string
	n    int
}

func main() {
	var p unsafe.Pointer
	fmt.Println(atomic.LoadPointer(&p) == nil)
	a := &config{"a", 1}
	b := &config{"b", 2}
	atomic.StorePointer(&p, unsafe.Pointer(a))
	fmt.Println((*config)(atomic.LoadPointer(&p)).name)
	fmt.Println(atomic.CompareAndSwapPointer(&p, unsafe.Pointer(b), unsafe.Pointer(b)))
	fmt.Println(atomic.CompareAndSwapPointer(&p, unsafe.Pointer(a), unsafe.Pointer(b)))
	old := atomic.SwapPointer(&p, unsafe.Pointer(a))
	fmt.Println((*config)(old).name, (*config)(atomic.LoadPointer(&p)).name)
	atomic.StorePointer(&p, nil)
	fmt.Println(atomic.LoadPointer(&p) == nil)

	// Readers see either the old or the new config completely while a writer replaces it.
	var cur unsafe.Pointer
	atomic.StorePointer(&cur, unsafe.Pointer(&config{"v0", 0}))
	var wg sync.WaitGroup
	torn := 0
	var mu sync.Mutex
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				c := (*config)(atomic.LoadPointer(&cur))
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
			atomic.StorePointer(&cur, unsafe.Pointer(&config{fmt.Sprint("v", i), i}))
		}
	}()
	wg.Wait()
	fmt.Println("torn:", torn, "final:", (*config)(atomic.LoadPointer(&cur)).n)
}

// Output:
// true
// a
// false
// true
// b a
// true
// torn: 0 final: 2000
