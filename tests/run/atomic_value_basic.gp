// phase: 08

// atomic.Value: Load, Store, Swap and CompareAndSwap of values of one type.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type config struct {
	host string
	port int
}

func main() {
	var v atomic.Value
	fmt.Println(v.Load())
	v.Store("one")
	fmt.Println(v.Load())
	fmt.Println(v.Swap("two"), v.Load())
	fmt.Println(v.CompareAndSwap("nope", "x"), v.CompareAndSwap("two", "three"), v.Load())

	var c atomic.Value
	fmt.Println(c.Swap(config{"a", 1}))
	c.Store(config{"b", 2})
	fmt.Println(c.Load().(config).host, c.Load().(config).port)
	fmt.Println(c.CompareAndSwap(config{"b", 2}, config{"c", 3}), c.Load())
	fmt.Println(c.CompareAndSwap(config{"b", 2}, config{"d", 4}), c.Load())

	var first atomic.Value
	fmt.Println(first.CompareAndSwap(nil, 1), first.Load())
	var second atomic.Value
	fmt.Println(second.CompareAndSwap(1, 2), second.Load())

	// A reader and a writer: the reader only ever sees complete values.
	var cfg atomic.Value
	cfg.Store(config{"h0", 0})
	var wg sync.WaitGroup
	torn := 0
	var mu sync.Mutex
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				x := cfg.Load().(config)
				if x.host != fmt.Sprint("h", x.port) {
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
			cfg.Store(config{fmt.Sprint("h", i), i})
		}
	}()
	wg.Wait()
	fmt.Println("torn:", torn, cfg.Load())
}

// Output:
// <nil>
// one
// one two
// false true three
// <nil>
// b 2
// true {c 3}
// false {c 3}
// true 1
// false <nil>
// torn: 0 {h2000 2000}
