// bench: binarytrees
// category: cpu
// size: 18
// smoke: 10
// twin: yes
// about: binary-trees: allocate, walk and drop many small pointer trees (allocation and reference counting)
package main

import "fmt"

type node struct {
	left, right *node
}

func bottomUp(depth int) *node {
	if depth <= 0 {
		return &node{}
	}
	return &node{bottomUp(depth - 1), bottomUp(depth - 1)}
}

func (n *node) check() int {
	if n.left == nil {
		return 1
	}
	return 1 + n.left.check() + n.right.check()
}

func main() {
	maxDepth := benchN
	minDepth := 4
	stretch := maxDepth + 1
	fmt.Printf("stretch tree of depth %d check: %d\n", stretch, bottomUp(stretch).check())
	long := bottomUp(maxDepth)
	for d := minDepth; d <= maxDepth; d += 4 {
		iters := 1 << uint(maxDepth-d+minDepth)
		check := 0
		for i := 0; i < iters; i++ {
			check += bottomUp(d).check()
		}
		fmt.Printf("%d trees of depth %d check: %d\n", iters, d, check)
	}
	fmt.Printf("long lived tree of depth %d check: %d\n", maxDepth, long.check())
}
