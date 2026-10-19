// phase: 08

// Main returns while other goroutines wait for ever: the program ends with exit code 0, nothing
// is reported.
package main

import "fmt"

func main() {
	block := make(chan int)
	for i := 0; i < 3; i++ {
		go func() { <-block }()
	}
	go func() { select {} }()
	var nilch chan int
	go func() { nilch <- 1 }()
	fmt.Println("main returns")
}

// Output:
// main returns
