// phase: 08

// OnceValue and OnceValues called from many goroutines run the function once and share its result;
// a panic is replayed to every caller.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var calls atomic.Int32
	val := sync.OnceValue(func() int { calls.Add(1); return 42 })
	vals := sync.OnceValues(func() (string, error) { calls.Add(1); return "x", fmt.Errorf("err") })
	var wg sync.WaitGroup
	var sum atomic.Int32
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := vals()
			if s == "x" && err != nil {
				sum.Add(int32(val()))
			}
		}()
	}
	wg.Wait()
	fmt.Println(calls.Load(), sum.Load())

	boom := sync.OnceFunc(func() { panic("boom") })
	for i := 0; i < 3; i++ {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			boom()
		}()
	}
}

// Output:
// 2 4200
// recovered: boom
// recovered: boom
// recovered: boom
