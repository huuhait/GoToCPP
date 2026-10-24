// phase: 08
// flags: -release -trace
// ExitCode: 2
// Stderr: (?s)^panic: runtime error: integer divide by zero\n\ngoroutine 1 \[running\]:\nmain\.div\([^\n]*\)\n\t[^\n]*\.g[op]:13 \+0x[0-9a-f]+\nmain\.main\(\)\n\t[^\n]*\.g[op]:16 \+0x[0-9a-f]+\n$

// -release -trace: a main.main of one line that g++'s -flto inlines into C++'s main (through the
// runtime's run_main) still has its frame, main.main() with an offset as in Go: the trace ends at
// C++'s main only after the Go functions inlined into it. div is //go:noinline, which gocpp
// follows (GP_NOINLINE), so it has a frame and an offset of its own.
package main

//go:noinline
func div(a, b int) int { return a / b }

func main() {
	println(div(1, 0))
}

// Output:
