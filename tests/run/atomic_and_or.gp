// phase: 08

// And and Or (Go 1.23) return the OLD value, in both APIs.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var a int32 = 0b1100
	fmt.Println(atomic.AndInt32(&a, 0b0110), a)
	fmt.Println(atomic.OrInt32(&a, 0b0011), a)
	var b uint32 = 0xff00
	fmt.Println(atomic.AndUint32(&b, 0x0ff0), b)
	fmt.Println(atomic.OrUint32(&b, 0x000f), b)
	var c int64 = -1
	fmt.Println(atomic.AndInt64(&c, 0xf0), c)
	fmt.Println(atomic.OrInt64(&c, 0x0f), c)
	var d uint64 = 1 << 40
	fmt.Println(atomic.OrUint64(&d, 1), atomic.AndUint64(&d, 1<<40), d)
	var e uintptr = 6
	fmt.Println(atomic.AndUintptr(&e, 3), atomic.OrUintptr(&e, 8), e)

	var t atomic.Uint32
	fmt.Println(t.Or(5), t.And(4), t.Load())
	var ti atomic.Int64
	fmt.Println(ti.Or(-8), ti.And(0xff), ti.Load())

	// A bit set shared by goroutines: each sets its own bit, with Or.
	var set atomic.Uint64
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			set.Or(1 << i)
		}()
	}
	wg.Wait()
	fmt.Printf("%x\n", set.Load())
	// ... and each clears its own bit with And.
	for i := 0; i < 64; i += 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			set.And(^(uint64(1) << i))
		}()
	}
	wg.Wait()
	fmt.Printf("%x\n", set.Load())
}

// Output:
// 12 4
// 4 7
// 65280 3840
// 3840 3855
// -1 240
// 240 255
// 1099511627776 1099511627777 1099511627776
// 6 2 10
// 0 5 4
// 0 -8 248
// ffffffffffffffff
// aaaaaaaaaaaaaaaa
