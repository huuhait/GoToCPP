// bench: goroutine_spawn
// category: runtime
// size: 500000
// smoke: 1000
// twin: no
// about: spawn goroutines in batches of 100 and join them through a buffered channel
package main

import "fmt"

func main() {
	total := 0
	done := make(chan int, 100)
	for round := 0; round < benchN/100; round++ {
		for i := 0; i < 100; i++ {
			go func() { done <- i * round }()
		}
		for i := 0; i < 100; i++ {
			total += <-done
		}
	}
	fmt.Println(total)
}
