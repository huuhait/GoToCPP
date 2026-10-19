// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_send_then_write\.g[op]:(24|31)\b

// The sender writes a value after it sent it: the receiver reads it at the same time.
package main

import "fmt"

type Msg struct{ text string }

func main() {
	ch := make(chan *Msg)
	done := make(chan int)
	go func() {
		m := <-ch
		n := 0
		for k := 0; k < 100; k++ {
			n += len(m.text)
		}
		done <- n
	}()
	m := &Msg{text: "hello"}
	ch <- m
	for k := 0; k < 100; k++ {
		m.text = "changed"
	}
	<-done
	fmt.Println("done")
}

// Output:
// done
