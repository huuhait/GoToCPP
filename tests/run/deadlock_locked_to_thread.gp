// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive, locked to thread\]:\n
// Stderr: \ngoroutine \d+ \[chan receive\]:\n(?:.+\n\t.+\n)+created by time\.goFunc\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A goroutine that called runtime.LockOSThread is listed "locked to thread" (every goroutine has a
// thread of its own here, but Go's report says so), and the goroutine that an AfterFunc started
// is "created by time.goFunc".
package main

import (
	"fmt"
	"runtime"
	"time"
)

func main() {
	runtime.LockOSThread()
	done := make(chan int)
	time.AfterFunc(10*time.Millisecond, func() {
		fmt.Println("the timer function waits")
		<-done
	})
	time.Sleep(50 * time.Millisecond)
	<-done
}

// Output:
// the timer function waits
