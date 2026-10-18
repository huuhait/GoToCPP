// phase: 08

// Goroutines that are still blocked when main returns do not keep the program alive.
package main

import "fmt"

func main() {
	forever := make(chan int)
	for i := 0; i < 20; i++ {
		go func() { <-forever }()
	}
	var nilc chan int
	go func() { nilc <- 1 }()
	fmt.Println("main returns while 21 goroutines are blocked")
}

// Output:
// main returns while 21 goroutines are blocked
