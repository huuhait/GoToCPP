// phase: 08
// ExitCode: 2
// Stderr: ^panic: non-positive interval for NewTicker\n\ngoroutine 1 \[running\]:\n

// NewTicker panics when the interval is not positive.
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("start")
	time.NewTicker(0)
	fmt.Println("not reached")
}

// Output:
// start
