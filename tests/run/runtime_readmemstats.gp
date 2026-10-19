// phase: 08

// ReadMemStats fills the fields that a program reads: the heap in use, the memory from the
// system. (gocpp reports what its allocator says; the counts of collections are 0.)
package main

import (
	"fmt"
	"runtime"
)

func main() {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	keep := make([][]byte, 0, 100)
	for i := 0; i < 100; i++ {
		keep = append(keep, make([]byte, 100000))
	}
	runtime.ReadMemStats(&after)
	fmt.Println(before.Alloc > 0, before.Sys > 0, before.HeapAlloc > 0)
	fmt.Println(after.Alloc > before.Alloc, after.Sys >= before.Sys)
	fmt.Println(len(keep), after.Alloc-before.Alloc >= 5000000)
}

// Output:
// true true true
// true true
// 100 true
