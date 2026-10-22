// phase: 08

// ReadMemStats: TotalAlloc is never below Alloc and does not decrease, and each runtime.GC() is a
// completed cycle in NumGC. (The numbers differ from run to run, so the program prints only what
// holds in Go and in gocpp.)
package main

import (
	"fmt"
	"runtime"
)

var keep [][]byte

func main() {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 20; i++ {
		keep = append(keep, make([]byte, 100000))
	}
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&after)
	fmt.Println("alloc grew:", after.Alloc > before.Alloc)
	fmt.Println("total >= alloc:", after.TotalAlloc >= after.Alloc)
	fmt.Println("total did not shrink:", after.TotalAlloc >= before.TotalAlloc)
	fmt.Println("two cycles:", after.NumGC >= before.NumGC+2)
	keep = nil
	runtime.GC()
	var last runtime.MemStats
	runtime.ReadMemStats(&last)
	fmt.Println("total after free >= before:", last.TotalAlloc >= after.TotalAlloc)
	fmt.Println("three cycles:", last.NumGC >= before.NumGC+3)
	fmt.Println("forced <= cycles:", last.NumForcedGC <= last.NumGC)
	fmt.Println("three forced:", last.NumForcedGC >= before.NumForcedGC+3)
}

// Output:
// alloc grew: true
// total >= alloc: true
// total did not shrink: true
// two cycles: true
// total after free >= before: true
// three cycles: true
// forced <= cycles: true
// three forced: true
