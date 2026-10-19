// phase: 08

// Adds wrap around like the machine integers; decrementing an unsigned value uses ^uint32(0).
package main

import (
	"fmt"
	"sync/atomic"
)

func main() {
	var u uint32 = 5
	fmt.Println(atomic.AddUint32(&u, ^uint32(0)))     // u - 1
	fmt.Println(atomic.AddUint32(&u, ^uint32(3-1)))   // u - 3
	fmt.Println(atomic.AddUint32(&u, ^uint32(0)))     // wraps
	var i int32 = 1<<31-1
	fmt.Println(atomic.AddInt32(&i, 1))
	var j int64 = -1<<63
	fmt.Println(atomic.AddInt64(&j, -1))
	var k uint64 = 1<<64-1
	fmt.Println(atomic.AddUint64(&k, 2))
	var up uintptr
	fmt.Println(atomic.AddUintptr(&up, ^uintptr(0)) == ^uintptr(0))

	var a atomic.Int32
	a.Store(1<<31-1)
	fmt.Println(a.Add(1), a.Add(-1))
	var b atomic.Uint64
	fmt.Println(b.Add(^uint64(0)), b.Add(2))
}

// Output:
// 4
// 1
// 0
// -2147483648
// 9223372036854775807
// 1
// true
// -2147483648 2147483647
// 18446744073709551615 1
