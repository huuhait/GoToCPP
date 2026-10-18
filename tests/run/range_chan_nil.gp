// phase: 08

// Ranging over a nil channel blocks for ever; other goroutines go on, and the program ends when main
// returns.
package main

import "fmt"

func main() {
	var nilc chan int
	started := make(chan bool)
	go func() {
		started <- true
		for v := range nilc {
			fmt.Println("never", v)
		}
		fmt.Println("never ended")
	}()
	<-started
	c := make(chan int)
	go func() {
		for i := 0; i < 3; i++ {
			c <- i
		}
		close(c)
	}()
	for v := range c {
		fmt.Println(v)
	}
	fmt.Println("main done")
}

// Output:
// 0
// 1
// 2
// main done
