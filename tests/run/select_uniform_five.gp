// phase: 08

// Five ready cases (receives and sends mixed), plus a default that must never run while a case is
// ready: 10000 selections, each case chosen in more than 5% of them.
package main

import "fmt"

func main() {
	r1, r2, r3 := make(chan int, 1), make(chan int, 1), make(chan int, 1)
	s1, s2 := make(chan int, 1), make(chan int, 1)
	r1 <- 1
	r2 <- 1
	r3 <- 1
	var n [5]int
	defaults := 0
	const trials = 10000
	for i := 0; i < trials; i++ {
		select {
		case <-r1:
			n[0]++
			r1 <- 1
		case <-r2:
			n[1]++
			r2 <- 1
		case <-r3:
			n[2]++
			r3 <- 1
		case s1 <- 1:
			n[3]++
			<-s1
		case s2 <- 1:
			n[4]++
			<-s2
		default:
			defaults++
		}
	}
	ok := true
	total := 0
	for _, c := range n {
		total += c
		if c <= trials/20 {
			ok = false
		}
	}
	fmt.Println(total, defaults, ok)
}

// Output:
// 10000 0 true
