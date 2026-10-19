// phase: 08

// A done channel counts the goroutines, like a WaitGroup would.
package main

import "fmt"

var total int

func main() {
	lock := make(chan bool, 1)
	done := make(chan bool, 16)
	for i := 1; i <= 16; i++ {
		go func() {
			lock <- true
			total += i
			<-lock
			done <- true
		}()
	}
	for i := 0; i < 16; i++ {
		<-done
	}
	fmt.Println(total)
}

// Output:
// 136
