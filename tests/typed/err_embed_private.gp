// phase: 12
// error: s\.secret undefined \(the member secret of (embed_errors\.)?Locked is private\)

// A private member is not promoted: the selector is an error that names the class.
package main

import "./embed_errors.hpp"

type S struct{ embed_errors.Locked }

func main() {
	var s S
	_ = s.open_field
	_ = s.visible()
	_ = s.secret
}
