// phase: 08
// ExitCode: 2
// Stderr: (?s)^panic: boom\n\ngoroutine \d+ \[running\]:\n.*\ngoroutine 1 \[chan receive\]:\n.*\ngoroutine \d+ \[chan receive\]:\n.*main\.main\.func1.*\ncreated by main\.main in goroutine 1
// Stderr: (?s)goroutine \d+ \[chan receive\]:\n.*main\.waiter
// Stderr: (?s)goroutine \d+ \[sleep\]:\ntime\.Sleep\(

// debug.SetTraceback("all") makes an unrecovered panic print every goroutine, not only the one
// that panics, as Go does (and as testing does for the panic of a timeout): the goroutines that
// are parked on a channel give their stacks, and one that sleeps shows [sleep] and its stack, as
// in Go. The panic waits, with runtime.Stack(buf, true), until the three goroutines that wait on
// the channel (main and two more) are parked and the sleeper sleeps; a bound of one minute ends a
// run in which they never are.
package main

import (
	"runtime"
	"runtime/debug"
	"time"
)

func waiter(never chan int) {
	<-never
}

// count says how many times mark is in text.
func count(text, mark string) int {
	found := 0
	for i := 0; i+len(mark) <= len(text); i++ {
		if text[i:i+len(mark)] == mark {
			found++
		}
	}
	return found
}

// parked reports whether at least want goroutines show "[chan receive]:" in the trace of all, and
// one shows "[sleep]:" with its stack.
func parked(want int) bool {
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	text := string(buf[:n])
	return count(text, "[chan receive]:") >= want && count(text, "[sleep]:\ntime.Sleep(") >= 1
}

func main() {
	debug.SetTraceback("all")
	never := make(chan int)
	ready := make(chan int)
	go func() {
		ready <- 1
		<-never
	}()
	go waiter(never)
	go func() {
		time.Sleep(time.Hour)
	}()
	<-ready
	go func() {
		deadline := time.Now().Add(time.Minute)
		for !parked(3) {
			if time.Now().After(deadline) {
				panic("the goroutines never parked")
			}
			time.Sleep(time.Millisecond)
		}
		panic("boom")
	}()
	<-never
}

// Output:
