// phase: 12
// error: s\.open_field undefined \(the member open_field of (embed_errors\.)?Hidden is private\)

// The members of a private base are not accessible through the derived class.
package main

import "./embed_errors.hpp"

type S struct{ embed_errors.Hidden }

func main() {
	var s S
	_ = s.open_field
}
