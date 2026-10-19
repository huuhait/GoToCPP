// phase: 08

// The function-style API: Add, Load, Store, Swap and CompareAndSwap on plain variables, struct
// fields and slice elements, from many goroutines.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type stats struct {
	hits   int64
	misses uint32
	flag   int32
	big    uint64
	ptr    uintptr
}

func main() {
	var s stats
	arr := make([]int32, 4)
	var plain int64
	var wg sync.WaitGroup
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				atomic.AddInt64(&s.hits, 1)
				atomic.AddUint32(&s.misses, 2)
				atomic.AddUint64(&s.big, 3)
				atomic.AddUintptr(&s.ptr, 4)
				atomic.AddInt32(&arr[i%4], 1)
				atomic.AddInt64(&plain, -1)
			}
		}()
	}
	wg.Wait()
	fmt.Println(s.hits, s.misses, s.big, s.ptr, arr, plain)

	atomic.StoreInt32(&s.flag, 5)
	fmt.Println(atomic.LoadInt32(&s.flag), atomic.SwapInt32(&s.flag, 9), atomic.LoadInt32(&s.flag))
	fmt.Println(atomic.CompareAndSwapInt32(&s.flag, 9, 10), atomic.CompareAndSwapInt32(&s.flag, 9, 11), s.flag)
	atomic.StoreUint64(&s.big, 1<<63)
	fmt.Println(atomic.LoadUint64(&s.big), atomic.SwapUint64(&s.big, 7), atomic.CompareAndSwapUint64(&s.big, 7, 8), s.big)
	atomic.StoreUintptr(&s.ptr, 77)
	fmt.Println(atomic.LoadUintptr(&s.ptr), atomic.SwapUintptr(&s.ptr, 78), atomic.CompareAndSwapUintptr(&s.ptr, 78, 79), s.ptr)
	atomic.StoreUint32(&s.misses, 1)
	fmt.Println(atomic.LoadUint32(&s.misses), atomic.SwapUint32(&s.misses, 2), atomic.CompareAndSwapUint32(&s.misses, 2, 3), s.misses)
	atomic.StoreInt64(&plain, 6)
	fmt.Println(atomic.LoadInt64(&plain), atomic.SwapInt64(&plain, 7), atomic.CompareAndSwapInt64(&plain, 7, 8), plain)
}

// Output:
// 50000 100000 150000 200000 [12500 12500 12500 12500] -50000
// 5 5 9
// true false 10
// 9223372036854775808 9223372036854775808 true 8
// 77 77 true 79
// 1 1 true 3
// 6 6 true 8
