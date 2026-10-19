// bench: nqueens
// category: cpu
// size: 14
// smoke: 7
// twin: yes
// about: n-queens: count all solutions with a recursive backtracking search with bit masks
package main

import "fmt"

func solve(n, row int, cols, d1, d2 uint32) int {
	if row == n {
		return 1
	}
	count := 0
	avail := ^(cols | d1 | d2) & (1<<uint(n) - 1)
	for avail != 0 {
		bit := avail & -avail
		avail ^= bit
		count += solve(n, row+1, cols|bit, (d1|bit)<<1, (d2|bit)>>1)
	}
	return count
}

func main() {
	total := 0
	for n := 4; n <= benchN; n++ {
		s := solve(n, 0, 0, 0, 0)
		total += s
		fmt.Println(n, s)
	}
	fmt.Println(total)
}
