// phase: 08
// ExitCode: 2
// Stderr: ^panic: goroutine boom\n\ngoroutine \d+ \[running\]:\n

// An unrecovered panic in a goroutine ends the program with code 2, and the output that was
// printed before it is not lost.
package main

import "fmt"

func main() {
	for i := 1; i <= 5; i++ {
		fmt.Println("line", i)
	}
	block := make(chan int)
	go func() {
		panic("goroutine boom")
	}()
	<-block
}

// Output:
// line 1
// line 2
// line 3
// line 4
// line 5
