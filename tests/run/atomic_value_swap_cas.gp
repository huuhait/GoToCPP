// phase: 08

// atomic.Value Swap and CompareAndSwap: the stored type is fixed by the first Store, nil panics, and
// CompareAndSwap succeeds for exactly one of many competing goroutines.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func try(f func()) {
	defer func() { fmt.Println("panic:", recover()) }()
	f()
}

func main() {
	var v atomic.Value
	fmt.Println(v.Load())
	fmt.Println(v.Swap("a"), v.Load())
	fmt.Println(v.Swap("b"), v.Load())
	fmt.Println(v.CompareAndSwap("b", "c"), v.CompareAndSwap("b", "d"), v.Load())
	try(func() { v.Store(1) })
	try(func() { v.Store(nil) })
	try(func() { v.Swap(2) })
	try(func() { v.CompareAndSwap("c", 3) })
	try(func() { v.CompareAndSwap(nil, "x") })

	var w atomic.Value
	w.Store(0)
	var wg sync.WaitGroup
	var winners atomic.Int32
	for g := 1; g <= 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if w.CompareAndSwap(0, g) {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	fmt.Println(winners.Load(), w.Load().(int) > 0)
}

// Output:
// <nil>
// <nil> a
// a b
// true false c
// panic: sync/atomic: store of inconsistently typed value into Value
// panic: sync/atomic: store of nil value into Value
// panic: sync/atomic: swap of inconsistently typed value into Value
// panic: sync/atomic: compare and swap of inconsistently typed values
// panic: <nil>
// 1 true
