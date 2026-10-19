// phase: 08

// Goexit leaves named results and deferred modifications alone: they live in the goroutine's
// frames, which are unwound normally.
package main

import (
	"fmt"
	"runtime"
)

type Resource struct{ name string }

func (r *Resource) Close() { fmt.Println("closing", r.name) }

func use(out chan<- string) (n int) {
	r := &Resource{"file"}
	defer r.Close()
	defer func() { n *= 10; out <- fmt.Sprint("n at the end: ", n) }()
	n = 4
	runtime.Goexit()
	return 99
}

func main() {
	out := make(chan string, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		use(out)
	}()
	<-finished
	fmt.Println(<-out)
}

// Output:
// closing file
// n at the end: 40
