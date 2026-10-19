package main

import "<map>"
import "<string>"

type Node struct{ ID int }

func main() {
	var m std.map[std.string, Node]
	m.insert(1)
	_ = m.nosuch
}
