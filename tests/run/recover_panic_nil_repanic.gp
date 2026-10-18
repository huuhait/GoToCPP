// phase: 05
// ExitCode: 2
// Stderr: ^panic: panic called with nil argument \[recovered, repanicked\]\n\ngoroutine 1 \[running\]:\n

package main

func main() {
	defer func() {
		r := recover()
		panic(r)
	}()
	panic(nil)
}

// Output:
