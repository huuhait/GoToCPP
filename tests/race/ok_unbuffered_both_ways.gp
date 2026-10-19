// phase: 08

// An unbuffered receive orders what the receiver did before it with what the sender does after the send.
package main

import "fmt"

var x int

func main() {
	ch := make(chan bool)
	done := make(chan bool)
	go func() {
		x = 1
		<-ch
		done <- true
	}()
	ch <- true
	x = 2
	<-done
	fmt.Println(x)
}

// Output:
// 2
