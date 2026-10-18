// phase: 08

// break leaves the select, not the enclosing for; continue goes to the next iteration of the loop;
// a labeled break and continue go to the outer loop.
package main

import "fmt"

func main() {
	c := make(chan int, 10)
	for i := 0; i < 10; i++ {
		c <- i
	}
	close(c)

	evens, odds := 0, 0
	iters := 0
	for {
		iters++
		select {
		case v, ok := <-c:
			if !ok {
				fmt.Println("closed after", iters, "iterations")
				goto out
			}
			if v%2 == 0 {
				evens += v
				break
			}
			odds += v
			continue
		}
		fmt.Println("after select (even only)")
	}
out:
	fmt.Println(evens, odds)

	d := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		d <- i
	}
	sum := 0
loop:
	for {
		select {
		case v := <-d:
			if v == 4 {
				break loop
			}
			sum += v
		default:
			fmt.Println("empty")
			break loop
		}
	}
	fmt.Println(sum, len(d))

	e := make(chan int, 6)
	for i := 0; i < 6; i++ {
		e <- i
	}
	close(e)
	count := 0
outer:
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			select {
			case v, ok := <-e:
				if !ok {
					break outer
				}
				if v%2 == 1 {
					continue outer
				}
				count++
			}
		}
	}
	fmt.Println(count, len(e))
}

// Output:
// after select (even only)
// after select (even only)
// after select (even only)
// after select (even only)
// after select (even only)
// closed after 11 iterations
// 20 25
// 6 1
// 3 0
