// phase: 08

// A loop variable is a new variable in each iteration (Go 1.22), so each closure has its own.
package main

import "fmt"

func main() {
	out := make([]int, 5)
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			j := i * 2
			out[i] = j
			done <- true
		}()
	}
	for i := 0; i < 5; i++ {
		<-done
	}
	fmt.Println(out)
}

// Output:
// [0 2 4 6 8]
