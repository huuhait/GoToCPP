// phase: 08
// ExitCode: 2
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^panic: runtime error: invalid memory address or nil pointer dereference
// Stderr: (?m)^==================\npanic: runtime error
// Stderr: race_then_panic\.g[op]:(24)\b

// A panic after a race: the report is written before the panic, the program ends with the panic's exit code 2.
package main

import "fmt"

var x int

type T struct{ n int }

func main() {
	done := make(chan bool)
	var p *T
	for g := 0; g < 2; g++ {
		go func() {
			for i := 0; i < 100; i++ {
				x++
			}
			done <- true
		}()
	}
	<-done
	<-done
	fmt.Println("before the panic")
	fmt.Println(p.n)
}

// Output:
// before the panic
