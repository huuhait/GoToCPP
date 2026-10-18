// phase: 08

// Deferred calls of a goroutine run when its function returns, last first; the goroutine's
// results (named results) can be changed by them.
package main

import "fmt"

func work(id int, out chan<- string) (r int) {
	defer func() { out <- fmt.Sprint("worker ", id, " result ", r) }()
	defer func() { r *= 2 }()
	defer func() { r += 1 }()
	return id * 10
}

func main() {
	out := make(chan string)
	for id := 1; id <= 3; id++ {
		go work(id, out)
		fmt.Println(<-out)
	}
}

// Output:
// worker 1 result 22
// worker 2 result 42
// worker 3 result 62
