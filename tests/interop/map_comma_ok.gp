// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./map_comma_ok\.gp:15:2: assignment mismatch: 2 variables but ages\["bob"\] is 1 value: a C\+\+ map has no comma-ok form, use ages\.find\(k\) != ages\.end\(\) or ages\.count\(k\) == 1$

// A std::map has no comma-ok form: m[k] is C++'s operator[], one value.
package main

import (
	"<map>"
	"<string>"
)

func main() {
	var ages std.map[std.string, int]
	v, ok := ages["bob"]
	_, _ = v, ok
}

// Output:
