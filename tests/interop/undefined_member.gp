// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./undefined_member\.gp:13:17: undefined: std\.chrono\.steady_clok \(the imported headers declare no ::std::chrono::steady_clok\)$

// A name inside a namespace that the headers don't declare.
package main

import (
	"<chrono>"
)

func main() {
	_ = std.chrono.steady_clok.now()
}

// Output:
