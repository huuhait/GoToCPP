package main

import "<utility>"

type size int

func main() {
	var p std.pair[int, int]
	_ = p.size()
	var s size = 2
	_ = s
}
