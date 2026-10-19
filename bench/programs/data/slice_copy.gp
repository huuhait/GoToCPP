// bench: slice_copy
// category: data
// size: 600000
// smoke: 300
// twin: no
// about: copy of slices, re-slicing windows and row copies of a 2-D grid
package main

import "fmt"

func main() {
	n := benchN
	a := make([]int, 4096)
	b := make([]int, 4096)
	for i := range a {
		a[i] = i * 31
	}
	var sum uint64
	for r := 0; r < n; r++ {
		off := r & 1023
		copy(b[off:], a[:len(a)-off])
		a[r&4095] += r
		sum += uint64(b[(r*7)&4095])
	}

	grid := make([][]int, 256)
	for i := range grid {
		grid[i] = make([]int, 256)
		for j := range grid[i] {
			grid[i][j] = i ^ j
		}
	}
	for r := 0; r < n*4; r++ {
		copy(grid[(r+1)&255], grid[r&255])
		grid[r&255][r&255] += r
	}
	var gsum uint64
	for i := range grid {
		gsum += uint64(grid[i][(i*5)&255])
	}

	var wsum uint64
	for r := 0; r < n*20; r++ {
		i := r & 2047
		w := a[i : i+16 : i+16]
		w = w[2:10]
		wsum += uint64(w[0]) + uint64(w[len(w)-1]) + uint64(cap(w))
	}
	fmt.Println(sum, gsum, wsum)
}
