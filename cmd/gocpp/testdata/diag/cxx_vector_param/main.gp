package main

import "./takes.h"

func main() {
	words := []string{"a", "b"}
	takes.total(words)
	takes.total(map[string]int{})
	var v std.vector[std.string]
	takes.total(v)
}
