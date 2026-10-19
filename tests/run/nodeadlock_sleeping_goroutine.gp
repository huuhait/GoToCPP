// phase: 08

// A goroutine that sleeps is not asleep for ever: main waits for its message.
package main

import (
	"fmt"
	"time"
)

func main() {
	ch := make(chan string)
	go func() {
		time.Sleep(30 * time.Millisecond)
		ch <- "woke up"
	}()
	fmt.Println(<-ch)
	var done = make(chan struct{})
	go func() {
		for i := 0; i < 3; i++ {
			time.Sleep(5 * time.Millisecond)
		}
		close(done)
	}()
	<-done
	fmt.Println("done")
}

// Output:
// woke up
// done
