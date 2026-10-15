// phase: 04
// Stderr: ^println 1\nprintln 2\n$

// C's stdio has a buffer of its own when stdout is a pipe (as here: the test harness reads it
// through one), and gocpp's fmt writes without one. fmt.Print* and println flush C's stdout and
// stderr before they write (Phase 04 STATE, the decision of job 4.2), so the lines come out in
// the program's order. The last printf is flushed at the end of the program.
package main

import (
	"fmt"

	"<cstdio>"
)

func main() {
	cstdio.printf("printf 1\n")
	fmt.Println("fmt 1")
	cstdio.printf("printf %d\n", 2)
	cstdio.puts("puts 3")
	fmt.Print("fmt 2\n")
	cstdio.printf("printf 4 ")
	fmt.Printf("fmt %d\n", 3)
	println("println 1")
	cstdio.fputs("fputs 5\n", cstdio.stdout)
	println("println 2")
	fmt.Println("fmt 4")
	cstdio.printf("printf 6, at the end\n")
}

// Output:
// printf 1
// fmt 1
// printf 2
// puts 3
// fmt 2
// printf 4 fmt 3
// fputs 5
// fmt 4
// printf 6, at the end
