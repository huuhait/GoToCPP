// phase: 08

// A goroutine that recovers from a send on a closed channel and reports it.
package main

import "fmt"

func trySend(c chan int, v int, report chan<- string) {
	defer func() {
		if r := recover(); r != nil {
			report <- fmt.Sprint("send ", v, " failed: ", r)
		}
	}()
	c <- v
	report <- fmt.Sprint("send ", v, " ok")
}

func main() {
	c := make(chan int, 1)
	report := make(chan string)
	go trySend(c, 1, report)
	fmt.Println(<-report)
	close(c)
	go trySend(c, 2, report)
	fmt.Println(<-report)
	fmt.Println(<-c, len(c))
	v, ok := <-c
	fmt.Println(v, ok)
}

// Output:
// send 1 ok
// send 2 failed: send on closed channel
// 1 0
// 0 false
