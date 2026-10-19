package main

import "./node.h"

type Node struct{ Name string }

func main() {
	var n node.Node
	_ = n
	_ = Node{}
	node.take(Node{}, "x")
	node.takeP(&Node{})
}
