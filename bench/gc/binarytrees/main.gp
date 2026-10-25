// gcbench: binarytrees
// about: allocate, walk and free many small pointer trees (the classic binary-trees workload), split over T workers
//
// The same file is valid Go and gp: tools/gcbench builds it as main.go with the go
// command and as main.gp with gocpp, so the two engines run the same algorithm.
//
//	binarytrees THREADS [DEPTH] [-quiet]
//
// It prints one deterministic line, "checksum N", which must be identical in Go and
// in gp. Without -quiet it also prints the elapsed time (which is not compared).
//
// The classic benchmark builds a stretch tree of depth DEPTH+1, one long-lived tree
// of depth DEPTH, and 1<<(DEPTH-d+4) trees of each depth d from 4 to DEPTH. A whole
// tree cannot be split over workers, so each of the two big trees is cut at level 4:
// its 15 upper nodes are counted directly and its 16 subtrees are work items. Every
// work item is then an independent "build a tree of depth d and check it", and the
// items are handed to the workers by their estimated cost (the node count), not by
// their number, so a worker with a deep tree does fewer trees. The checksum is the
// same for every thread count.
package main

import (
	"os"
	"time"
)

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

// atoi parses a non-negative decimal integer without importing strconv (the GC
// engine's binarytrees should pull as little of the standard library as it can).
func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// bucket is a run of identical work items: count trees of the given depth.
// start is the cumulative cost of the buckets before it, weight the cost of
// one item.
type bucket struct {
	depth, count  int
	start, weight int64
}

// itemCost estimates the work of one item of the given depth: the number of
// nodes of the tree (2^(depth+1)-1, close enough to 2^depth for sharing).
func itemCost(depth int) int64 { return 1 << uint(depth) }

func main() {
	threads, depth := 1, 18
	quiet := false
	pos := 0
	for _, a := range os.Args[1:] {
		if a == "-quiet" {
			quiet = true
			continue
		}
		if pos == 0 {
			threads = atoi(a)
		} else if pos == 1 {
			depth = atoi(a)
		}
		pos++
	}
	if threads < 1 {
		threads = 1
	}
	if depth < 4 {
		depth = 4
	}

	// The two big trees cut at level 4 (15 upper nodes each), then the classic
	// depth loop.
	buckets := []bucket{
		{depth - 3, 16, 0, 0}, // the stretch tree (depth+1): its 16 subtrees of depth-3
		{depth - 4, 16, 0, 0}, // the long-lived tree (depth): its 16 subtrees of depth-4
	}
	for d := 4; d <= depth; d += 4 {
		buckets = append(buckets, bucket{d, 1 << uint(depth-d+4), 0, 0})
	}
	var total int64
	for i := range buckets {
		buckets[i].weight = itemCost(buckets[i].depth)
		buckets[i].start = total
		total += int64(buckets[i].count) * buckets[i].weight
	}

	// work builds and checks the items whose cost falls in [lo, hi) and returns
	// their check sum.
	work := func(lo, hi int64) int {
		sum := 0
		for _, b := range buckets {
			span := int64(b.count) * b.weight
			a, e := lo-b.start, hi-b.start
			if a < 0 {
				a = 0
			}
			if e > span {
				e = span
			}
			if a >= e {
				continue
			}
			// The items covered by the cost range [a, e).
			first := (a + b.weight - 1) / b.weight
			last := (e + b.weight - 1) / b.weight
			for i := first; i < last; i++ {
				sum += bottomUp(b.depth).check()
			}
		}
		return sum
	}

	start := time.Now()
	sum := 30 // the 15 upper nodes of each of the two big trees
	if threads == 1 {
		sum += work(0, total)
	} else {
		results := make(chan int, threads-1)
		for g := 1; g < threads; g++ {
			go func(g int) {
				results <- work(int64(g)*total/int64(threads), int64(g+1)*total/int64(threads))
			}(g)
		}
		sum += work(0, total/int64(threads))
		for g := 1; g < threads; g++ {
			sum += <-results
		}
	}
	elapsed := time.Since(start)

	println("checksum", sum)
	if !quiet {
		println("elapsed_ms", int64(elapsed)/1000000)
	}
}
