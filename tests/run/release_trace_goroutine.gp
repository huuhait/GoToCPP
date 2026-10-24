// phase: 08
// flags: -release -trace
// ExitCode: 2
// Stderr: (?s)^panic: too big\n\ngoroutine \d+ \[running\]:\nmain\.\(\*T\)\.Check\(\.\.\.\)\n\t[^\n]*\.g[op]:16(?: \+0x[0-9a-f]+)?\nmain\.worker\([^\n]*\)\n\t[^\n]*\.g[op]:21(?: \+0x[0-9a-f]+)?\ncreated by main\.main in goroutine 1\n\t[^\n]*\.g[op]:26 \+0x[0-9a-f]+\n$

// -release -trace: the panic of a goroutine prints its trace with the .gp lines, the inlined
// method as a frame of its own, and "created by" at the line of the go statement.
package main

import "time"

type T struct{ n int }

func (t *T) Check() {
	if t.n > 2 {
		panic("too big")
	}
}

func worker(t *T) {
	t.Check()
	time.Sleep(time.Millisecond)
}

func main() {
	go worker(&T{n: 3})
	time.Sleep(time.Minute)
}

// Output:
