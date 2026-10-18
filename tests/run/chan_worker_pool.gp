// phase: 08

// A worker pool: jobs go in on one channel, results come back tagged with the job number; the main
// goroutine puts them in order.
package main

import "fmt"

type result struct {
	job   int
	value string
}

func isPrime(n int) bool {
	if n < 2 {
		return false
	}
	for d := 2; d*d <= n; d++ {
		if n%d == 0 {
			return false
		}
	}
	return true
}

func main() {
	const jobs = 40
	in := make(chan int, jobs)
	out := make(chan result, jobs)
	for w := 0; w < 5; w++ {
		go func() {
			for {
				n, ok := <-in
				if !ok {
					return
				}
				v := "composite"
				if isPrime(n) {
					v = "prime"
				}
				out <- result{n, v}
			}
		}()
	}
	for n := 1; n <= jobs; n++ {
		in <- n
	}
	close(in)
	table := make([]string, jobs+1)
	for i := 0; i < jobs; i++ {
		r := <-out
		table[r.job] = r.value
	}
	primes := 0
	for n := 1; n <= jobs; n++ {
		if table[n] == "prime" {
			fmt.Print(n, ",")
			primes++
		}
	}
	fmt.Println("\ncount", primes)
}

// Output:
// 2,3,5,7,11,13,17,19,23,29,31,37,
// count 12
