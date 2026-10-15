// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./undefined_type\.gp:15:11: undefined: std\.vectr \(the imported headers declare no ::std::vectr\)$

// A C++ name that the headers don't declare is an error at its place: only the C++ compiler's
// kind probe can tell, so the build reports it (gocpp check can't).
package main

import (
	"fmt"

	"<vector>"
)

var v std.vectr[int]

func main() {
	fmt.Println("unreachable")
}

// Output:
