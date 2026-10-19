package main

import "./node.h"

type Node struct{ Name string }
type Item struct{ Name string }
type Point struct{ X int }

func main() {
	var n node.Node[int]
	node.takeI(n); _ = Node{}
	node.takeI(Node{}); _ = Item{}
	node.ns.takeP(Point{})
	node.ns.takeP(5); _ = Point{}
	node.Point("x"); _ = Point{}
	node.takeI(Item{})
}
