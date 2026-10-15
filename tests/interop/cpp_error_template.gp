// phase: 04

// An error deep in a system header's template is reported at the .gp line that instantiated
// it, with the first error's message; the template backtrace is left out.
package main

import (
	"<algorithm>"
	"<vector>"
)

func main() {
	std.sort(C.int(1), C.int(2)) // ERROR "C\+\+: no type named 'value_type' in '(struct )?std::iterator_traits<int>'"
	v := std.vector[C.int]()
	std.sort(v, v) // ERROR "C\+\+: (no match for 'operator-'|invalid operands to binary expression).*'std::vector<int>' and 'std::vector<int>'"
}
