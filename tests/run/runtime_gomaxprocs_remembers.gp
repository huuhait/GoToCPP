// phase: 08

// GOMAXPROCS returns the previous setting and remembers the new one.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	orig := runtime.GOMAXPROCS(0)
	fmt.Println("default is positive:", orig >= 1)
	fmt.Println(runtime.GOMAXPROCS(3) == orig)
	fmt.Println(runtime.GOMAXPROCS(0))
	fmt.Println(runtime.GOMAXPROCS(5))
	fmt.Println(runtime.GOMAXPROCS(0))
	fmt.Println(runtime.GOMAXPROCS(orig))
	fmt.Println(runtime.GOMAXPROCS(0) == orig)
}

// Output:
// default is positive: true
// true
// 3
// 3
// 5
// 5
// true
