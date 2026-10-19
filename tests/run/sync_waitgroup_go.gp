// phase: 08

// WaitGroup.Go (Go 1.25) starts the goroutine, counting it, and calls Done when f returns, also
// when f panics and the panic is recovered inside it.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var wg sync.WaitGroup
	var total atomic.Int64
	for i := 1; i <= 100; i++ {
		wg.Go(func() {
			total.Add(int64(i))
		})
	}
	wg.Wait()
	fmt.Println("sum:", total.Load())

	var recovered atomic.Int32
	for i := 0; i < 5; i++ {
		wg.Go(func() {
			defer func() {
				if recover() != nil {
					recovered.Add(1)
				}
			}()
			panic("boom")
		})
	}
	wg.Wait()
	fmt.Println("recovered:", recovered.Load())

	// A nested Go from inside a goroutine, before the outer one is done.
	var inner atomic.Int32
	wg.Go(func() {
		for i := 0; i < 3; i++ {
			wg.Go(func() { inner.Add(1) })
		}
	})
	wg.Wait()
	fmt.Println("inner:", inner.Load())
}

// Output:
// sum: 5050
// recovered: 5
// inner: 3
