// phase: 08

// A channel of closures: a goroutine sends functions, another one runs them in order; a closure
// captures its own loop variable (Go 1.22 per-iteration variables).
package main

import "fmt"

func main() {
	work := make(chan func() string, 3)
	out := make(chan string)
	go func() {
		for f := range work {
			out <- f()
		}
		close(out)
	}()
	go func() {
		for i := 0; i < 6; i++ {
			work <- func() string { return fmt.Sprint("job ", i, " squared ", i*i) }
		}
		close(work)
	}()
	for s := range out {
		fmt.Println(s)
	}
}

// Output:
// job 0 squared 0
// job 1 squared 1
// job 2 squared 4
// job 3 squared 9
// job 4 squared 16
// job 5 squared 25
