// phase: 08
// flags: -release -trace
// ExitCode: 2
// Stderr: (?s)^panic: from goroutine\n\ngoroutine \d+ \[running\]:\nmain\.main\.func1\((?:\.\.\.)?\)\n\t[^\n]*\.g[op]:16(?: \+0x[0-9a-f]+)?\ncreated by main\.main in goroutine 1\n\t[^\n]*\.g[op]:15 \+0x[0-9a-f]+\n$

// -release -trace: a func literal that a go statement runs is a frame of its own, main.main.func1,
// with both compilers. g++ would inline its lambda into the runtime's code that starts it, and its
// -g1 line tables have no linkage name for an inlined lambda, so gocpp keeps the lambdas of go and
// defer statements out of line there (GP_TRACE_FRAME, ADR-136): main.main.func1() with its offset,
// as Go. clang++ inlines it and names it from its line tables: main.main.func1(...), no offset.
package main

func main() {
	c := make(chan int)
	go func() {
		panic("from goroutine")
	}()
	<-c
}

// Output:
