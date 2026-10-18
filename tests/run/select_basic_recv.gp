// phase: 08

// select with receive cases: only the channel that has a value is ready; the value is bound
// per case (v := <-c, v, ok := <-c, a bare <-c).
package main

import "fmt"

func main() {
	a := make(chan int, 1)
	b := make(chan string, 1)
	c := make(chan struct{}, 1)

	a <- 41
	select {
	case v := <-a:
		fmt.Println("a", v+1)
	case s := <-b:
		fmt.Println("b", s)
	case <-c:
		fmt.Println("c")
	}

	b <- "bee"
	select {
	case v := <-a:
		fmt.Println("a", v)
	case s, ok := <-b:
		fmt.Println("b", s, ok)
	case <-c:
		fmt.Println("c")
	}

	c <- struct{}{}
	select {
	case v := <-a:
		fmt.Println("a", v)
	case s := <-b:
		fmt.Println("b", s)
	case <-c:
		fmt.Println("c")
	}
	fmt.Println(len(a), len(b), len(c))
}

// Output:
// a 42
// b bee true
// c
// 0 0 0
