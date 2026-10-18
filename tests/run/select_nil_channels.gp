// phase: 08

// A nil channel is never ready in a select, for a receive or a send; with a default, the
// default runs; the other case is chosen.
package main

import "fmt"

func main() {
	var nilc chan int
	ready := make(chan int, 1)
	ready <- 5
	readies := 0
	for i := 0; i < 50; i++ {
		select {
		case v := <-nilc:
			fmt.Println("nil recv", v)
		case nilc <- 1:
			fmt.Println("nil send")
		case v := <-ready:
			readies += v
			ready <- 5
		}
	}
	fmt.Println("ready", readies)
	select {
	case <-nilc:
		fmt.Println("no")
	case nilc <- 2:
		fmt.Println("no")
	default:
		fmt.Println("default")
	}

	// Disabling a case by setting its channel to nil, the classic merge.
	a, b := make(chan int), make(chan int)
	go func() {
		for i := 0; i < 3; i++ {
			a <- i
		}
		close(a)
	}()
	go func() {
		for i := 10; i < 12; i++ {
			b <- i
		}
		close(b)
	}()
	sum, n := 0, 0
	for a != nil || b != nil {
		select {
		case v, ok := <-a:
			if !ok {
				a = nil
				continue
			}
			sum += v
			n++
		case v, ok := <-b:
			if !ok {
				b = nil
				continue
			}
			sum += v
			n++
		}
	}
	fmt.Println(sum, n)
}


// Output:
// ready 250
// default
// 24 5
