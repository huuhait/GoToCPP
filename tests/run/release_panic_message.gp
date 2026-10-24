// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: ^panic: boom\n$

// A -release program that panics prints only the panic's message and exits with 2 (ADR-136, the
// amendment of 2026-10-23): no "goroutine 1 [running]:", no trace. Real Go prints the same with
// GOTRACEBACK=none, which TestRunVsGo sets for a -release test.
package main

import "fmt"

func boom(n int) {
	if n > 0 {
		panic("boom")
	}
}

func main() {
	fmt.Println("before")
	boom(1)
}

// Output:
// before
