// phase: 08

// A channel with one slot is a lock: the counter is only touched with it held.
package main

import "fmt"

var counter int

func main() {
	lock := make(chan struct{}, 1)
	done := make(chan bool)
	for g := 0; g < 8; g++ {
		go func() {
			for i := 0; i < 100; i++ {
				lock <- struct{}{}
				counter++
				<-lock
			}
			done <- true
		}()
	}
	for g := 0; g < 8; g++ {
		<-done
	}
	fmt.Println(counter)
}

// Output:
// 800
