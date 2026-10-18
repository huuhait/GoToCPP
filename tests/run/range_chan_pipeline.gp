// phase: 08

// A pipeline of range loops: generator, squarer, filter, sink; and a worker pool whose results are
// sorted.
package main

import "fmt"

func gen(n int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for i := 1; i <= n; i++ {
			out <- i
		}
	}()
	return out
}

func square(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for v := range in {
			out <- v * v
		}
	}()
	return out
}

func odd(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for v := range in {
			if v%2 == 1 {
				out <- v
			}
		}
	}()
	return out
}

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
	sum := 0
	for v := range odd(square(gen(20))) {
		sum += v
	}
	fmt.Println(sum)

	jobs := make(chan int, 100)
	results := make(chan int, 100)
	done := make(chan bool)
	for w := 0; w < 4; w++ {
		go func() {
			for j := range jobs {
				results <- j * 2
			}
			done <- true
		}()
	}
	for i := 1; i <= 50; i++ {
		jobs <- i
	}
	close(jobs)
	for w := 0; w < 4; w++ {
		<-done
	}
	close(results)
	var all []int
	for r := range results {
		all = append(all, r)
	}
	sortInts(all)
	fmt.Println(len(all), all[0], all[len(all)-1])
}

// Output:
// 1330
// 50 2 100
