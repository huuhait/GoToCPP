// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A Timer that was stopped cannot wake the receive on its channel.
package main

import (
	"fmt"
	"time"
)

func main() {
	t := time.NewTimer(time.Hour)
	fmt.Println("stopped:", t.Stop())
	<-t.C
}

// Output:
// stopped: true
