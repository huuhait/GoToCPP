// phase: 12
// error: ambiguous selector s\.size \(std\.vector\[int\]\.size and (embed_errors\.)?Twin\.size\)

// Go's rule: two embedded classes with a member of the name at the same depth make it ambiguous.
// Naming the field (s.vector.size()) is the way out.
package main

import (
	"<vector>"
	"./embed_errors.hpp"
)

type S struct {
	std.vector[int]
	embed_errors.Twin
}

func main() {
	var s S
	_ = s.vector.size()
	_ = s.size()
}
