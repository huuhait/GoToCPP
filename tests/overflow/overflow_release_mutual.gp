// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$

// Endless recursion ends with Go's "fatal error: stack overflow" and exit code 2: a mutual
// recursion of tail calls (f calls g, g calls f), which a compiler would make jumps. In a -release
// build the functions that can recurse keep their calls (GP_RECURSIVE and GP_RECURSION_MARK,
// internal/lower/recursion.go); the report is the message lines only.
package main

func f(n int) int {
	if n < 0 {
		return 0
	}
	return g(n + 1)
}

func g(n int) int {
	if n < 0 {
		return 1
	}
	return f(n + 1)
}

func main() {
	println(f(0))
}

// Output:
