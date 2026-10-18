// phase: 08

// defer inside a range loop over a channel, and a select inside the loop body.
package main

import "fmt"

func drain(c chan int, stop chan bool) {
	for v := range c {
		defer fmt.Println("deferred", v)
		select {
		case <-stop:
			fmt.Println("stop at", v)
			return
		default:
			fmt.Println("keep", v)
		}
	}
	fmt.Println("drained")
}

func main() {
	c := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		c <- i
	}
	close(c)
	stop := make(chan bool, 1)
	go func() {}()
	drain(c, stop)
	d := make(chan int, 3)
	d <- 1
	d <- 2
	d <- 3
	close(d)
	stop <- true
	drain(d, stop)
}

// Output:
// keep 1
// keep 2
// keep 3
// keep 4
// keep 5
// drained
// deferred 5
// deferred 4
// deferred 3
// deferred 2
// deferred 1
// stop at 1
// deferred 1
