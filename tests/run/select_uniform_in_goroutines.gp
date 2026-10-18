// phase: 08

// The choice is random per goroutine and per call: eight goroutines each make 2000 selections
// between two always-ready channels; every goroutine sees both channels chosen often, and the
// same channel is not chosen in a fixed pattern (no run of 64 equal choices).
package main

import "fmt"

func main() {
	const workers = 8
	const trials = 2000
	type result struct {
		balanced bool
		longest  int
	}
	out := make(chan result, workers)
	for w := 0; w < workers; w++ {
		go func() {
			a := make(chan int, 1)
			b := make(chan int, 1)
			a <- 1
			b <- 1
			na, nb := 0, 0
			last, run, longest := -1, 0, 0
			for i := 0; i < trials; i++ {
				which := 0
				select {
				case <-a:
					na++
					a <- 1
				case <-b:
					nb++
					which = 1
					b <- 1
				}
				if which == last {
					run++
				} else {
					run = 1
					last = which
				}
				longest = max(longest, run)
			}
			out <- result{na > trials/10 && nb > trials/10, longest}
		}()
	}
	allBalanced, noLongRun := true, true
	for w := 0; w < workers; w++ {
		r := <-out
		allBalanced = allBalanced && r.balanced
		noLongRun = noLongRun && r.longest < 64
	}
	fmt.Println(allBalanced, noLongRun)
}

// Output:
// true true
