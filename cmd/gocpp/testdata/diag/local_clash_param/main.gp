package main

import "./node.h"

type Node struct{ Name string }

func main() {
	node.take2(5, 5); _ = Node{}
	node.take(Node{})
	node.over(5, 5, 5)
	_ = Node{}; node.over("x")
	var m map[string]Node
	node.take2(1, m)
}
