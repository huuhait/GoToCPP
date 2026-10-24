// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$

// Endless recursion ends with Go's "fatal error: stack overflow" and exit code 2: a recursion with
// an accumulator (return f(n+1) + x), which clang would make a loop. In a -release build the
// functions that can recurse keep their calls (GP_RECURSIVE and GP_RECURSION_MARK,
// internal/lower/recursion.go); the report is the message lines only.
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
