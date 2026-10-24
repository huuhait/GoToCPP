// phase: 09
// flags: -release -trace
// ExitCode: 2
// Stderr: (?s)^panic: boom\n\ngoroutine 1 \[running\]:\nmain\.main\(\)\n\t[^\n]*\.g[op]:\d+ \+0x[0-9a-f]+\n\ngoroutine \d+ \[chan receive\]:\nmain\.waiter\([^\n]*\)\n\t[^\n]*\ncreated by main\.main in goroutine 1\n\t[^\n]*\.g[op]:\d+ \+0x[0-9a-f]+\n$

// -release -trace honors the traceback level as a debug build does: with debug.SetTraceback("all")
// a panic prints the other goroutines too, each with its stack and "created by". main waits, with
// runtime.Stack(buf, true), until the waiter is parked; a bound of one minute ends a run in which
// it never is.
package main

import (
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

func waiter(never chan int) {
	<-never
}

func main() {
	go waiter(make(chan int))
	buf := make([]byte, 1<<16)
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		if strings.Contains(string(buf[:runtime.Stack(buf, true)]), "[chan receive]:") {
			break
		}
		time.Sleep(time.Millisecond)
	}
	debug.SetTraceback("all")
	panic("boom")
}

// Output:
