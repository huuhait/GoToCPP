// bench: slice_append
// category: data
// size: 5000000
// smoke: 5000
// twin: yes
// about: append growth of []int and []struct, and many short slices
package main

import "fmt"

type rec struct {
	id  int
	val uint64
	tag int32
}

func main() {
	n := benchN
	var xs []int
	for i := 0; i < n; i++ {
		xs = append(xs, i*3)
	}
	var sum uint64
	for _, x := range xs {
		sum += uint64(x)
	}

	var rs []rec
	for i := 0; i < n/2; i++ {
		rs = append(rs, rec{id: i, val: uint64(i) * 7, tag: int32(i & 15)})
	}
	var rsum uint64
	for i := range rs {
		rsum += rs[i].val + uint64(rs[i].tag)
	}

	// many short slices: grow from empty to 20 elements
	var lsum uint64
	for r := 0; r < n/20; r++ {
		var s []int
		for j := 0; j < 20; j++ {
			s = append(s, r+j)
		}
		lsum += uint64(s[19]) + uint64(len(s))
	}
	fmt.Println(len(xs), sum, len(rs), rsum, lsum)
}
