package main

import "<vector>"

func main() {
	var v std.vector[int]
	v.push_back("x")
	_ = v.size(1)
}
