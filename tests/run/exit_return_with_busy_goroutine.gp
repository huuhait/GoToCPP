// phase: 08

// main returns while another goroutine is in a loop that never ends: the program still ends at
// once with code 0.
package main

import "fmt"

func main() {
	running := make(chan struct{})
	go func() {
		close(running)
		n := 0
		for {
			n++
		}
	}()
	<-running
	fmt.Println("main returns while a goroutine spins")
}

// Output:
// main returns while a goroutine spins
