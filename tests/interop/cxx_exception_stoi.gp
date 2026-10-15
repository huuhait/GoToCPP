// phase: 04
// ExitCode: 2
// Stderr: ^panic: C\+\+ exception std::invalid_argument: [^\n]*stoi[^\n]*\n\ngoroutine 1 \[running\]:\nmain\.parse\(\.\.\.\)\n\t/.+/cxx_exception_stoi\.gp:16 \+0x[0-9a-f]+\nmain\.main\(\)\n\t/.+/cxx_exception_stoi\.gp:21 \+0x[0-9a-f]+\n$

// A C++ standard function that throws, called directly: std::stoi of a string that is no
// number throws std::invalid_argument.
package main

import (
	"fmt"

	"<string>"
)

func parse(s string) int {
	return int(std.stoi(std.string(s)))
}

func main() {
	fmt.Println(parse("42"))
	fmt.Println(parse("x"))
}

// Output:
// 42
