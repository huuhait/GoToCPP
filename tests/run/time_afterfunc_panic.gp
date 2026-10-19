// phase: 08
// ExitCode: 2
// Stderr: ^panic: boom in AfterFunc\n\ngoroutine \d+ \[running\]:\n

// A panic in the function of AfterFunc ends the program, as the panic of any goroutine does.
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("start")
	time.AfterFunc(5*time.Millisecond, func() { panic("boom in AfterFunc") })
	time.Sleep(10 * time.Second)
	fmt.Println("not reached")
}

// Output:
// start
