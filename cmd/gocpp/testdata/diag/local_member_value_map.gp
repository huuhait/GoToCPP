package main

import "<map>"

type value struct{}

func main() {
	var m std.map[int, int]
	_ = m.value()
	_ = value{}
}
