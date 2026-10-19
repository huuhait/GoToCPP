// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A range loop over a channel that the producer forgets to close.
package main

import "fmt"

func main() {
	ch := make(chan int)
	go func() {
		for i := 1; i <= 3; i++ {
			ch <- i
		}
		// no close(ch)
	}()
	for v := range ch {
		fmt.Println("got", v)
	}
	fmt.Println("not reached")
}

// Output:
// got 1
// got 2
// got 3
