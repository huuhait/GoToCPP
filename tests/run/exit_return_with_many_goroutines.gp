// phase: 08

// A thousand goroutines are blocked when main returns: the program ends with code 0.
package main

import "fmt"

func main() {
	never := make(chan int)
	started := make(chan struct{}, 1000)
	for i := 0; i < 1000; i++ {
		go func() {
			started <- struct{}{}
			<-never
		}()
	}
	for i := 0; i < 1000; i++ {
		<-started
	}
	fmt.Println("1000 goroutines are blocked")
}

// Output:
// 1000 goroutines are blocked
