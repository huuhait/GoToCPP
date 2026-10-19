// phase: 08

// Many goroutines end with Goexit at different depths; every deferred call runs once.
package main

import (
	"fmt"
	"runtime"
)

func dive(n int, ran chan<- int) {
	defer func() { ran <- n }()
	if n == 0 {
		runtime.Goexit()
	}
	dive(n-1, ran)
}

func main() {
	ran := make(chan int, 100)
	for d := 1; d <= 10; d++ {
		go dive(d, ran)
	}
	total := 0
	count := 0
	for range 10 * 11 / 2 + 10 {
		total += <-ran
		count++
	}
	fmt.Println(count, total)
	fmt.Println("goroutines left:", func() int {
		for runtime.NumGoroutine() > 1 {
			runtime.Gosched()
		}
		return runtime.NumGoroutine()
	}())
}

// Output:
// 65 220
// goroutines left: 1
