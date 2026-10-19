// phase: 08

// NumCPU is at least 1 and does not change; the default GOMAXPROCS is at least 1.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	n := runtime.NumCPU()
	fmt.Println(n >= 1, runtime.NumCPU() == n, runtime.GOMAXPROCS(0) >= 1)
	runtime.GOMAXPROCS(n + 3)
	fmt.Println(runtime.NumCPU() == n)
}

// Output:
// true true true
// true
