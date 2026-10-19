// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_nested_struct\.g[op]:(26)\b

// Two goroutines write a field of a struct inside a struct.
package main

import "fmt"

type Inner struct{ n int }
type Outer struct {
	in  Inner
	tag string
}

func main() {
	o := &Outer{tag: "x"}
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			o.in.n = i
			done <- true
		}()
	}
	<-done
	<-done
	fmt.Println("done", o.tag)
}

// Output:
// done x
