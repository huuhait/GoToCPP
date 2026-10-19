// phase: 08
// ExitCode: 2
// Stderr: ^panic: main boom\n\ngoroutine 1 \[running\]:\n

// A panic in main runs main's deferred calls, then ends the program with code 2, with the
// output flushed and the other goroutines left alone.
package main

import "fmt"

func main() {
	never := make(chan int)
	go func() { <-never }()
	defer fmt.Println("deferred call runs before the crash")
	fmt.Println("before the panic")
	panic("main boom")
}

// Output:
// before the panic
// deferred call runs before the crash
