// phase: 08

// What main did before a go statement is visible in the goroutine.
package main

import "fmt"

type Job struct {
	name string
	n    []int
}

func main() {
	j := &Job{name: "j"}
	j.n = append(j.n, 1, 2, 3)
	done := make(chan int)
	go func() {
		done <- len(j.n) + len(j.name)
	}()
	fmt.Println(<-done)
}

// Output:
// 4
