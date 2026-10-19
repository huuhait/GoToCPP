// phase: 08

// The typed API: Int32, Int64, Uint32, Uint64, Uintptr with every method.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var i32 atomic.Int32
	var i64 atomic.Int64
	var u32 atomic.Uint32
	var u64 atomic.Uint64
	var up atomic.Uintptr
	fmt.Println(i32.Load(), i64.Load(), u32.Load(), u64.Load(), up.Load())
	i32.Store(-5)
	i64.Store(-6)
	u32.Store(7)
	u64.Store(8)
	up.Store(9)
	fmt.Println(i32.Load(), i64.Load(), u32.Load(), u64.Load(), up.Load())
	fmt.Println(i32.Swap(1), i64.Swap(2), u32.Swap(3), u64.Swap(4), up.Swap(5))
	fmt.Println(i32.CompareAndSwap(1, 10), i32.CompareAndSwap(1, 11), i32.Load())
	fmt.Println(i64.CompareAndSwap(2, 20), i64.CompareAndSwap(2, 21), i64.Load())
	fmt.Println(u32.CompareAndSwap(3, 30), u32.CompareAndSwap(3, 31), u32.Load())
	fmt.Println(u64.CompareAndSwap(4, 40), u64.CompareAndSwap(4, 41), u64.Load())
	fmt.Println(up.CompareAndSwap(5, 50), up.CompareAndSwap(5, 51), up.Load())
	fmt.Println(i32.Add(5), i64.Add(-30), u32.Add(1), u64.Add(2), up.Add(3))

	var wg sync.WaitGroup
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				i32.Add(1)
				i64.Add(2)
				u32.Add(3)
				u64.Add(4)
				up.Add(5)
			}
		}()
	}
	wg.Wait()
	fmt.Println(i32.Load(), i64.Load(), u32.Load(), u64.Load(), up.Load())

	// A typed value inside a struct, and a pointer to it passed around.
	type S struct {
		name string
		n    atomic.Int64
	}
	s := &S{name: "s"}
	inc := func(c *atomic.Int64) { c.Add(10) }
	inc(&s.n)
	inc(&s.n)
	fmt.Println(s.name, s.n.Load())
}

// Output:
// 0 0 0 0 0
// -5 -6 7 8 9
// -5 -6 7 8 9
// true false 10
// true false 20
// true false 30
// true false 40
// true false 50
// 15 -10 31 42 53
// 32015 63990 96031 128042 160053
// s 20
