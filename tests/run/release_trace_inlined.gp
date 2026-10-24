// phase: 09
// flags: -release -trace
// ExitCode: 2
// Stderr: (?s)^panic: runtime error: index out of range \[3\] with length 3\n\ngoroutine 1 \[running\]:\nmain\.get\(\.\.\.\)\n\t[^\n]*\.g[op]:19(?: \+0x[0-9a-f]+)?\nmain\.sum\(\.\.\.\)\n\t[^\n]*\.g[op]:25(?: \+0x[0-9a-f]+)?\nmain\.main\(\)\n\t[^\n]*\.g[op]:33 \+0x[0-9a-f]+\n$

// -release -trace (ADR-136, the amendment of 2026-10-23): the line tables stay, so a panic prints
// Go's trace with the .gp lines, and the calls that the compiler inlined are frames of their own,
// as in Go: main.get and main.sum are inlined into main.main (by g++, clang++ and Go), and each
// shows the line of its own code or call, not the line of the runtime's check that is inlined
// there too.
package main

import (
	"fmt"
	"os"
)

func get(xs []int, i int) int {
	return xs[i]
}

func sum(xs []int, n int) int {
	t := 0
	for i := 0; i < n; i++ {
		t += get(xs, i)
	}
	return t
}

func main() {
	xs := []int{1, 2, 3}
	n := len(os.Args) + 3
	fmt.Println(sum(xs, n))
}

// Output:
