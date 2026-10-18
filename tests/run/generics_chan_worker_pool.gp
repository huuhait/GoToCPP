// phase: 08

// A generic worker pool: jobs of any type in, results of any type out, several goroutines.
package main

import "fmt"

func Pool[J, R any](n int, jobs []J, work func(J) R) []R {
	in := make(chan int)
	done := make(chan bool)
	res := make([]R, len(jobs))
	for range n {
		go func() {
			for {
				i, ok := <-in
				if !ok {
					done <- true
					return
				}
				res[i] = work(jobs[i])
			}
		}()
	}
	for i := range jobs {
		in <- i
	}
	close(in)
	for range n {
		<-done
	}
	return res
}

func main() {
	fmt.Println(Pool(3, []int{1, 2, 3, 4, 5, 6}, func(i int) int { return i * i }))
	fmt.Println(Pool(2, []string{"a", "bb", "ccc"}, func(s string) int { return len(s) }))
	fmt.Println(Pool(4, []float64{1, 2}, func(f float64) string { return fmt.Sprint(f / 2) }))
	fmt.Println(len(Pool(2, []int(nil), func(i int) bool { return true })))
}

// Output:
// [1 4 9 16 25 36]
// [1 2 3]
// [0.5 1]
// 0
