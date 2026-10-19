// phase: 08
// ExitCode: 2
// Stderr: ^panic: time: Stop called on uninitialized Timer\n\ngoroutine 1 \[running\]:\n

// An uninitialized Timer (not made by NewTimer or AfterFunc) panics in Stop with Go's message.
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("start")
	var t time.Timer
	t.Stop()
	fmt.Println("not reached")
}

// Output:
// start
