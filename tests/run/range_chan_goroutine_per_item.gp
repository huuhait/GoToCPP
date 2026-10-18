// phase: 08

// One goroutine per received value, results gathered through another channel and summed.
package main

import "fmt"

func main() {
	jobs := make(chan int)
	results := make(chan int)
	go func() {
		for i := 1; i <= 200; i++ {
			jobs <- i
		}
		close(jobs)
	}()
	count := 0
	go func() {
		for j := range jobs {
			count++
			go func() { results <- j * j }()
		}
		results <- -1 // marker: no more jobs started
	}()
	sum, got, marker := 0, 0, false
	for !marker || got < count {
		r := <-results
		if r == -1 {
			marker = true
			continue
		}
		sum += r
		got++
		if marker && got == 200 {
			break
		}
	}
	fmt.Println(sum, got)
}

// Output:
// 2686700 200
