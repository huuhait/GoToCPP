// phase: 08

// A parallel merge sort: each half is sorted by its own goroutine and sent back on a channel.
package main

import "fmt"

func merge(a, b []int) []int {
	out := make([]int, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] <= b[j] {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}

func sortPar(xs []int, depth int, out chan<- []int) {
	if len(xs) <= 1 {
		out <- xs
		return
	}
	mid := len(xs) / 2
	if depth == 0 {
		var l, r []int
		lc, rc := make(chan []int, 1), make(chan []int, 1)
		sortPar(xs[:mid], 0, lc)
		sortPar(xs[mid:], 0, rc)
		l, r = <-lc, <-rc
		out <- merge(l, r)
		return
	}
	lc, rc := make(chan []int), make(chan []int)
	go sortPar(xs[:mid], depth-1, lc)
	go sortPar(xs[mid:], depth-1, rc)
	out <- merge(<-lc, <-rc)
}

func main() {
	xs := make([]int, 5000)
	seed := 12345
	for i := range xs {
		seed = (seed*1103515245 + 12345) % 2147483648
		xs[i] = seed % 100000
	}
	out := make(chan []int)
	go sortPar(xs, 4, out)
	sorted := <-out
	ok := len(sorted) == len(xs)
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1] > sorted[i] {
			ok = false
		}
	}
	fmt.Println(ok, sorted[0], sorted[len(sorted)-1], sorted[len(sorted)/2])
}

// Output:
// true 16 99992 50150
