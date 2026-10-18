// phase: 08

// The ring buffer wraps around many times: values stay in order whatever the fill level is.
package main

import "fmt"

func main() {
	c := make(chan int, 5)
	next, want := 0, 0
	bad := 0
	fill := func(n int) {
		for i := 0; i < n; i++ {
			c <- next
			next++
		}
	}
	drain := func(n int) {
		n = min(n, len(c))
		for i := 0; i < n; i++ {
			if v := <-c; v != want {
				bad++
			}
			want++
		}
	}
	for round := 0; round < 1000; round++ {
		n := round%5 + 1
		if len(c)+n > cap(c) {
			n = cap(c) - len(c)
		}
		fill(n)
		drain(round%3 + 1)
		if len(c) > 0 && round%7 == 0 {
			drain(len(c))
		}
	}
	drain(len(c))
	fmt.Println("sent", next, "received", want, "out of order", bad, "len", len(c))
}

// Output:
// sent 2325 received 2325 out of order 0 len 0
