// phase: 08
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$
// Stderr: (?m)^goroutine \d+ .*\[running\]:$

// Endless recursion ends with Go's "fatal error: stack overflow" and exit code 2: a recursion with
// an accumulator (return f(n+1) + x), which clang would make a loop. The debug build of
// overflow_release_accumulator.gp.
package main

var sink [64]byte

func f(n int) int {
	sink[n%64] = byte(n)
	return f(n+1) + int(sink[3])
}

func main() {
	println(f(0))
}

// Output:
