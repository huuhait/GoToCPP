// phase: 08
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$
// Stderr: (?m)^goroutine \d+ .*\[running\]:$

// Endless recursion ends with Go's "fatal error: stack overflow" and exit code 2: a recursion in
// tail position (return f(n + 1)), which a compiler would make a loop, or (clang, as it has no
// side effect) drop. The debug build of overflow_release_tail.gp.
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
