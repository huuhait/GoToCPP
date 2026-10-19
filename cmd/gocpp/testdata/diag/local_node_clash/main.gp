package main

import "./node.h"

type Node struct{ Name string }

func main() {
	var n node.Node
	node.takeStr(n)
	_ = Node{}
}
