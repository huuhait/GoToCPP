// phase: 08

// Several producers and several consumers on one channel; every item is consumed exactly once and
// the totals are exact.
package main

import "fmt"

func main() {
	const producers, perProducer, consumers = 6, 5000, 4
	items := make(chan int, 16)
	prodDone := make(chan bool)
	for p := 0; p < producers; p++ {
		go func() {
			for i := 0; i < perProducer; i++ {
				items <- p*perProducer + i
			}
			prodDone <- true
		}()
	}
	go func() {
		for p := 0; p < producers; p++ {
			<-prodDone
		}
		close(items)
	}()
	sums := make(chan [2]int)
	for c := 0; c < consumers; c++ {
		go func() {
			n, sum := 0, 0
			for {
				v, ok := <-items
				if !ok {
					break
				}
				n++
				sum += v
			}
			sums <- [2]int{n, sum}
		}()
	}
	n, sum := 0, 0
	for c := 0; c < consumers; c++ {
		x := <-sums
		n += x[0]
		sum += x[1]
	}
	total := producers * perProducer
	fmt.Println(n == total, sum == total*(total-1)/2, n, sum)
}

// Output:
// true true 30000 449985000
