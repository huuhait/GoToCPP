// phase: 09
// ExitCode: 2
// Stderr: (?s)^fatal error: sync: unlock of unlocked mutex\n\ngoroutine 1 \[running\]:\n.*\nmain\.main\(\)\n\t[^\n]*:\d+ \+0x[0-9a-f]+\n\ngoroutine \d+ \[chan receive\]:\nmain\.waiter\([^\n]*\)\n\t[^\n]*\ncreated by main\.main in goroutine 1\n\t[^\n]*:\d+ \+0x[0-9a-f]+\n\ngoroutine \d+ \[sleep\]:\n.*\ncreated by main\.main in goroutine 1\n\t[^\n]*:\d+ \+0x[0-9a-f]+\n$

// A fatal error prints every goroutine, also with the default GOTRACEBACK, as Go does for a throw
// (a panic prints only its own goroutine then): the one that waits on a channel and the one that
// sleeps, each with its stack and "created by". main waits, with runtime.Stack(buf, true), until
// both are parked; a bound of one minute ends a run in which they never are.
package main

import (
	"runtime"
	"strings"
	"sync"
	"time"
)

func waiter(never chan int) {
	<-never
}

func sleeper() {
	time.Sleep(time.Hour)
}

func main() {
	never := make(chan int)
	go waiter(never)
	go sleeper()
	buf := make([]byte, 1<<16)
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		text := string(buf[:runtime.Stack(buf, true)])
		if strings.Contains(text, "[chan receive]:") && strings.Contains(text, "[sleep]:") {
			break
		}
		time.Sleep(time.Millisecond)
	}
	var mu sync.Mutex
	mu.Unlock()
}

// Output:
