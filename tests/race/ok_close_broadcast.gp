// phase: 08

// A close orders the writes before it with the reads after the receive of every reader.
package main

import "fmt"

var config map[string]int

func main() {
	ready := make(chan struct{})
	done := make(chan int)
	for i := 0; i < 4; i++ {
		go func() {
			<-ready
			done <- config["n"]
		}()
	}
	config = map[string]int{"n": 7}
	close(ready)
	sum := 0
	for i := 0; i < 4; i++ {
		sum += <-done
	}
	fmt.Println(sum)
}

// Output:
// 28
