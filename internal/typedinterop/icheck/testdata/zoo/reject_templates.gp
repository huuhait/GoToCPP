// Template members and functions with the wrong arguments.
package main

import (
	"<algorithm>"
	"<string>"
	"<vector>"
)

func main() {
	var v std.vector[int]
	var s std.string
	_, _ = v, s
	v.push_back("x")
	v.push_back(1, 2)
	_ = std.min(1, "a")
	_ = v.sizee()
	s.push_back(1, 2, 3)
}
