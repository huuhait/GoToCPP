// phase: 12
// error: s\.gone is a deleted function of (embed_errors\.)?Locked

// A member function that is declared = delete is an error where it is named.
package main

import "./embed_errors.hpp"

type S struct{ embed_errors.Locked }

func main() {
	var s S
	s.gone()
}
