// phase: 08

// A function whose select has no default and every case returns needs no final return: select is a
// terminating statement.
package main

import "fmt"

func pick(a, b chan int) int {
	select {
	case v := <-a:
		return v
	case v := <-b:
		return -v
	}
}

func pickDefault(a chan int) string {
	select {
	case <-a:
		return "got"
	default:
		return "none"
	}
}

func forever(c chan int) int {
	for {
		select {
		case v := <-c:
			if v > 2 {
				return v
			}
		}
	}
}

func main() {
	a, b := make(chan int, 1), make(chan int, 1)
	a <- 5
	fmt.Println(pick(a, b))
	b <- 6
	fmt.Println(pick(a, b))
	fmt.Println(pickDefault(a))
	a <- 1
	fmt.Println(pickDefault(a))
	c := make(chan int, 5)
	for i := 0; i < 5; i++ {
		c <- i
	}
	r := forever(c)
	fmt.Println(r, len(c))
}

// Output:
// 5
// -6
// none
// got
// 3 1
