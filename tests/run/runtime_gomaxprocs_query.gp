// phase: 08

// A value below 1 only asks: it changes nothing. A goroutine sees the setting that main made.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(2)
	fmt.Println(runtime.GOMAXPROCS(-1), runtime.GOMAXPROCS(0), runtime.GOMAXPROCS(-100))
	got := make(chan int)
	go func() { got <- runtime.GOMAXPROCS(0) }()
	fmt.Println(<-got)
	go func() { got <- runtime.GOMAXPROCS(4) }()
	fmt.Println(<-got, runtime.GOMAXPROCS(0))
}

// Output:
// 2 2 2
// 2
// 2 4
