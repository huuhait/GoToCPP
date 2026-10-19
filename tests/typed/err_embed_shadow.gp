// phase: 12
// error: s\.size is the member of std\.vector\[int\], which is shallower than the Go member size \(Go's rule\): name the field, s\.vector\.size

// Go's depth rule: the class's member (depth 1) hides a Go field of an embedded struct (depth 2).
package main

import "<vector>"

type Deep struct{ size int }
type Mid struct{ Deep }

type S struct {
	std.vector[int]
	Mid
}

func main() {
	var s S
	s.Mid.Deep.size = 1
	_ = s.size
}
