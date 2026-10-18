// phase: 08

// A closed channel is always ready to receive: the zero value and ok == false, after the
// buffer is drained.
package main

import "fmt"

type pair struct {
	a int
	s string
}

func main() {
	c := make(chan int, 2)
	c <- 1
	c <- 2
	close(c)
	for i := 0; i < 4; i++ {
		select {
		case v, ok := <-c:
			fmt.Println(v, ok)
		}
	}
	p := make(chan pair)
	close(p)
	select {
	case v, ok := <-p:
		fmt.Printf("%+v %v\n", v, ok)
	}
	s := make(chan []int)
	close(s)
	select {
	case v, ok := <-s:
		fmt.Println(v == nil, len(v), ok)
	}
	other := make(chan int)
	closed := make(chan string)
	close(closed)
	for i := 0; i < 100; i++ {
		select {
		case <-other:
			fmt.Println("never")
		case v := <-closed:
			if v != "" {
				fmt.Println("never")
			}
		}
	}
	fmt.Println("closed wins")
}

// Output:
// 1 true
// 2 true
// 0 false
// 0 false
// {a:0 s:} false
// true 0 false
// closed wins
