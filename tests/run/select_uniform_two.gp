// phase: 08

// When several cases are ready, Go picks one uniformly at random. 4000 selections between two
// channels that are always ready: each is chosen in more than 10% of them (the chance of a fair
// select missing that is far below one in a billion).
package main

import "fmt"

func main() {
	a := make(chan int, 1)
	b := make(chan int, 1)
	a <- 1
	b <- 1
	na, nb := 0, 0
	const trials = 4000
	for i := 0; i < trials; i++ {
		select {
		case <-a:
			na++
			a <- 1
		case <-b:
			nb++
			b <- 1
		}
	}
	fmt.Println(na+nb, na > trials/10, nb > trials/10)
}

// Output:
// 4000 true true
