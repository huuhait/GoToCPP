// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$

// Endless recursion ends with Go's "fatal error: stack overflow" and exit code 2: a recursion in
// tail position (return f(n + 1)), which a compiler would make a loop, or (clang, as it has no
// side effect) drop. In a -release build the functions that can recurse keep their calls
// (GP_RECURSIVE and GP_RECURSION_MARK, internal/lower/recursion.go); the report is the message
// lines only.
package main

func f(n int) int {
	if n < 0 {
		return 0
	}
	return f(n + 1)
}

func main() {
	println(f(0))
}

// Output:
