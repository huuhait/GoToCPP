// phase: 03
// ExitCode: 2
// Stderr: ^panic: deep\n\ngoroutine 1 \[running\]:\nmain\.main\.func1\.1\((\.\.\.)?\)\n\t/.+/closures_trace\.g[op]:18( \+0x[0-9a-f]+)?\nmain\.apply\((\.\.\.)?\)\n\t/.+/closures_trace\.g[op]:12( \+0x[0-9a-f]+)?\nmain\.main\.func1\((\.\.\.|0x[0-9a-f?]+)?\)\n\t/.+/closures_trace\.g[op]:17( \+0x[0-9a-f]+)?\nmain\.main\(\)\n\t/.+/closures_trace\.g[op]:22 \+0x[0-9a-f]+\n$

// A panic in a closure: the trace names func literals as Go does, main.main.func1 for the first
// literal of main and main.main.func1.1 for the first literal in it.
package main

var run func(int)

func apply(f func()) {
	f()
}

func main() {
	run = func(n int) {
		apply(func() {
			panic("deep")
		})
	}
	_ = func() {}
	run(1)
}

// Output:
