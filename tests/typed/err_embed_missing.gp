// phase: 12
// error: s\.nosuch undefined \(type S has no field or method nosuch; std\.vector\[int\] has no member nosuch\)

// A member that the embedded class does not have is an error at the selector, before the C++ compile.
package main

import "<vector>"

type S struct{ std.vector[int] }

func main() {
	var s S
	s.push_back(1)
	s.nosuch()
}
