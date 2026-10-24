// phase: 08
// flags: -release -trace
// ExitCode: 2
// Stderr: (?s)^panic: first \[recovered\]\n\tpanic: again: first\n\ngoroutine 1 \[running\]:\n(?:main\.main\.func1\([^\n]*\)\n\t[^\n]*\.g[op]:18(?: \+0x[0-9a-f]+)?\n)?panic\([^\n]*\)\n\t[^\n]*\nmain\.main\(\)\n\t[^\n]*\.g[op]:20 \+0x[0-9a-f]+\n$

// -release -trace: a panic in a deferred call shows Go's shape, the deferred call, a panic frame
// and the panicking function. The deferred func literal is inlined into the runtime's code that
// runs it; clang++ shows it as main.main.func1(...), but g++'s -g1 line tables have no linkage
// name for a lambda, so with g++ the frame is missing (docs/spec/cpp-mapping.md).
package main

import "fmt"

func main() {
	defer func() {
		r := recover()
		fmt.Println("recovered:", r)
		panic(fmt.Sprint("again: ", r))
	}()
	panic("first")
}

// Output:
// recovered: first
