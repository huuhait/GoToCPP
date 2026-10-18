// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./builtin_len_error\.gp:16:18: invalid argument: v \(C/C\+\+ value\) for built-in len: call its C\+\+ function, v\.size\(\) or std\.size\(v\)$

// len, cap, min and max do not take C/C++ values: gocpp does not know the type in opaque mode.
package main

import (
	"fmt"

	"<vector>"
)

func main() {
	var v std.vector[int]
	fmt.Println(len(v))
}

// Output:
