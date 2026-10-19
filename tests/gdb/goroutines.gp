// phase: 08

// gp goroutines (the gdb command of runtime/debug/gocpp-gdb.py): goroutines that wait on a
// channel receive, a send, a select with cases and a select without. Each one reports to main
// before it waits; main then polls a channel until the debugger stops it (or 40 s pass), at a breakpoint
// that waits for all six to be blocked (goroutines.gdb), so the list never depends on timing.
package main

import "time"

func worker(in <-chan int, ready chan<- bool) {
	ready <- true
	for v := range in { // @worker
		_ = v
	}
}

func sender(out chan<- int, ready chan<- bool) {
	ready <- true
	out <- 1
}

func main() {
	in := make(chan int)
	out := make(chan int)
	done := make(chan bool)
	other := make(chan string)
	never := make(chan int)
	ready := make(chan bool)
	go worker(in, ready)
	go worker(in, ready)
	go func() {
		ready <- true
		<-done
	}()
	go sender(out, ready)
	go func() {
		ready <- true
		select {
		case <-done:
		case <-other:
		}
	}()
	go func() {
		ready <- true
		select {}
	}()
	for range 6 {
		<-ready
	}
	// (The breakpoint stops the loop. Without it, or when its condition is never true, the
	// program ends by itself after a while, so that a failing test fails fast.)
	for deadline := time.Now().Add(40 * time.Second); time.Now().Before(deadline); {
		poll(never)
	}
}

// poll receives from c if a value is ready, and doesn't wait.
func poll(c chan int) {
	select { // @poll
	case <-c:
	default:
	}
}
