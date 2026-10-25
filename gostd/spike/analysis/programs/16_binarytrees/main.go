// The GC engine's binarytrees: the classic allocate-and-walk benchmark, with no imports (the
// built-in println is the only output). The checksum of the serial run and of the run split
// over two goroutines through channels must be the same, and both are deterministic.
package main

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

const maxDepth = 14

// checkDepths adds the checks of the trees of depths lo..hi (inclusive), the way the classic
// benchmark's depth loop does.
func checkDepths(lo, hi int) int {
	total := 0
	for d := lo; d <= hi; d++ {
		n := 1 << uint(maxDepth-d+4)
		for i := 0; i < n; i++ {
			total += bottomUp(d).check()
		}
	}
	return total
}

func main() {
	serial := checkDepths(4, maxDepth)
	println("serial", serial)
	results := make(chan int, 2)
	mid := (4 + maxDepth) / 2
	go func() { results <- checkDepths(4, mid) }()
	go func() { results <- checkDepths(mid+1, maxDepth) }()
	parallel := <-results + <-results
	println("parallel", parallel)
	println("equal", serial == parallel)
}
