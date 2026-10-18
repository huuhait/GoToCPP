// phase: 08

// break and goto with labels around select: a labeled break leaves the select itself, a goto from a
// case jumps back to retry, a labeled continue goes to an outer loop.
package main

import "fmt"

func main() {
	c := make(chan int, 3)
	c <- 1
	c <- 2
	c <- 3
sel:
	select {
	case v := <-c:
		if v < 3 {
			fmt.Println("small", v)
			break sel
		}
		fmt.Println("big", v)
	}
	fmt.Println("after sel, left", len(c))

	tries := 0
retry:
	select {
	case v := <-c:
		fmt.Println("got", v)
		if v < 10 && tries < 2 {
			c <- 100
			tries++
			goto retry
		}
	default:
		fmt.Println("empty after", tries)
	}
	fmt.Println("len", len(c))
	for len(c) > 0 {
		select {
		case v := <-c:
			fmt.Println("rest", v)
		}
	}

	d := make(chan int, 6)
	for i := 0; i < 6; i++ {
		d <- i
	}
	n := 0
rows:
	for i := 0; i < 3; i++ {
		for j := 0; j < 5; j++ {
			select {
			case v := <-d:
				n += v
				if v%2 == 0 {
					continue rows
				}
				if v == 5 {
					break rows
				}
			default:
				break rows
			}
		}
	}
	fmt.Println(n, len(d))
}

// Output:
// small 1
// after sel, left 2
// got 2
// got 3
// got 100
// len 1
// rest 100
// 10 1
