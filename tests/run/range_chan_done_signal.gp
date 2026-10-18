// phase: 08

// chan struct{} and chan bool as signals; range over a channel that is closed by another goroutine
// after all values were sent; the loop sees every value exactly once.
package main

import "fmt"

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func main() {
	sig := make(chan struct{})
	go func() {
		for i := 0; i < 5; i++ {
			sig <- struct{}{}
		}
		close(sig)
	}()
	n := 0
	for range sig {
		n++
	}
	fmt.Println(n)

	// several producers, one closer
	out := make(chan int)
	fin := make(chan bool)
	const producers = 5
	for p := 0; p < producers; p++ {
		go func() {
			for i := 0; i < 20; i++ {
				out <- p*100 + i
			}
			fin <- true
		}()
	}
	go func() {
		for p := 0; p < producers; p++ {
			<-fin
		}
		close(out)
	}()
	var all []int
	for v := range out {
		all = append(all, v)
	}
	sortInts(all)
	fmt.Println(len(all), all[0], all[19], all[20], all[99])
}

// Output:
// 5
// 100 0 19 100 419
