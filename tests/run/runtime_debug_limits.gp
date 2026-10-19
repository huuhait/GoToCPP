// phase: 08

// debug.SetMemoryLimit, SetMaxStack, SetMaxThreads and SetPanicOnFault return the previous
// setting.
package main

import (
	"fmt"
	"runtime/debug"
)

const maxInt64 = 1<<63 - 1

func main() {
	fmt.Println(debug.SetMemoryLimit(-1) == maxInt64)
	fmt.Println(debug.SetMemoryLimit(1<<30) == maxInt64)
	fmt.Println(debug.SetMemoryLimit(-1) == 1<<30)
	debug.SetMemoryLimit(maxInt64)
	fmt.Println(debug.SetMaxThreads(5000))
	fmt.Println(debug.SetMaxThreads(10000))
	fmt.Println(debug.SetPanicOnFault(true))
	fmt.Println(debug.SetPanicOnFault(false))
	debug.FreeOSMemory()
	debug.SetTraceback("all")
	fmt.Println("ok")
}

// Output:
// true
// true
// true
// 10000
// 5000
// false
// true
// ok
