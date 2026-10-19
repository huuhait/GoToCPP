// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_byte_buffer\.g[op]:(20)\b

// Two goroutines write the same byte of a shared buffer.
package main

import "fmt"

func main() {
	buf := make([]byte, 16)
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			buf[0] = byte(i)
			done <- true
		}()
	}
	<-done
	<-done
	fmt.Println("done", len(buf))
}

// Output:
// done 16
