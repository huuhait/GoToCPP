// phase: 08

// The panics of channel operations are runtime errors with Go's text, and recover() sees them.
package main

import "fmt"

func try(name string, f func()) {
	defer func() {
		r := recover()
		err, isErr := r.(error)
		fmt.Printf("%s: %v (error: %v)\n", name, r, isErr)
		if isErr {
			fmt.Println("  ", err.Error())
		}
	}()
	f()
}

func main() {
	try("close nil", func() {
		var c chan int
		close(c)
	})
	try("close twice", func() {
		c := make(chan int)
		close(c)
		close(c)
	})
	try("send closed", func() {
		c := make(chan int, 1)
		close(c)
		c <- 1
	})
	try("make negative", func() {
		n := -1
		_ = make(chan int, n)
	})
	try("fine", func() {
		c := make(chan int, 1)
		c <- 1
		close(c)
		<-c
	})
}

// Output:
// close nil: close of nil channel (error: true)
//    close of nil channel
// close twice: close of closed channel (error: true)
//    close of closed channel
// send closed: send on closed channel (error: true)
//    send on closed channel
// make negative: makechan: size out of range (error: true)
//    makechan: size out of range
// fine: <nil> (error: false)
