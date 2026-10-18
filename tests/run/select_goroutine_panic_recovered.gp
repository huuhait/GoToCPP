// phase: 08

// A panic in a case body of a goroutine is recovered by a deferred function of that goroutine; the
// result is reported through a channel.
package main

import "fmt"

func worker(in chan int, out chan string) {
	defer func() {
		if r := recover(); r != nil {
			out <- fmt.Sprint("recovered: ", r)
		}
	}()
	for {
		select {
		case v := <-in:
			if v < 0 {
				panic(fmt.Sprint("negative ", v))
			}
			out <- fmt.Sprint("ok ", v)
		}
	}
}

func main() {
	in := make(chan int)
	out := make(chan string)
	go worker(in, out)
	for _, v := range []int{1, 2} {
		in <- v
		fmt.Println(<-out)
	}
	in <- -5
	fmt.Println(<-out)
}

// Output:
// ok 1
// ok 2
// recovered: negative -5
