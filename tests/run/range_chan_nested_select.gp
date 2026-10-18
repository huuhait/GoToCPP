// phase: 08

// range over a channel with a select in the body that also watches a quit channel, and a loop that
// ends because the channel closes before quit is used.
package main

import "fmt"

func consume(in <-chan int, quit <-chan bool) (sum int, quitEarly bool) {
	for v := range in {
		select {
		case <-quit:
			return sum, true
		default:
			sum += v
		}
	}
	return sum, false
}

func main() {
	in := make(chan int, 5)
	quit := make(chan bool, 1)
	for i := 1; i <= 5; i++ {
		in <- i
	}
	close(in)
	fmt.Println(consume(in, quit))

	in2 := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		in2 <- i
	}
	close(in2)
	quit <- true
	fmt.Println(consume(in2, quit))
	fmt.Println(len(in2))
}

// Output:
// 15 false
// 0 true
// 4
