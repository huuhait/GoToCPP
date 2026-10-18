// phase: 08

// break, continue, labels, goto and return inside a range over a channel.
package main

import "fmt"

func gen(n int) chan int {
	c := make(chan int, n)
	for i := 0; i < n; i++ {
		c <- i
	}
	close(c)
	return c
}

func firstOver(limit int) int {
	for v := range gen(20) {
		if v > limit {
			return v
		}
	}
	return -1
}

func main() {
	for v := range gen(10) {
		if v%2 == 1 {
			continue
		}
		if v > 6 {
			break
		}
		fmt.Println("even", v)
	}
	fmt.Println(firstOver(7), firstOver(100))

outer:
	for i := 0; i < 3; i++ {
		for v := range gen(5) {
			if v == 2 {
				continue outer
			}
			if i == 2 {
				break outer
			}
			fmt.Println(i, v)
		}
	}

	c := gen(6)
	for v := range c {
		if v == 3 {
			goto done
		}
		fmt.Println("g", v)
	}
	fmt.Println("not reached")
done:
	fmt.Println("done", len(c))

	// the rest of a channel after a break is still there
	d := gen(5)
	for v := range d {
		if v == 1 {
			break
		}
	}
	fmt.Println(len(d))
	for v := range d {
		fmt.Println("rest", v)
	}
}

// Output:
// even 0
// even 2
// even 4
// even 6
// 8 -1
// 0 0
// 0 1
// 1 0
// 1 1
// g 0
// g 1
// g 2
// done 2
// 3
// rest 2
// rest 3
// rest 4
