// phase: 02

// Collatz sequences: the steps for a few numbers, and the start below
// 100000 with the longest chain.
package main

import "fmt"

func steps(n uint64) int {
	count := 0
	for n != 1 {
		if n%2 == 0 {
			n /= 2
		} else {
			n = 3*n + 1
		}
		count++
	}
	return count
}

func peak(n uint64) uint64 {
	top := n
	for n != 1 {
		if n&1 == 1 {
			n = 3*n + 1
		} else {
			n >>= 1
		}
		if n > top {
			top = n
		}
	}
	return top
}

func main() {
	fmt.Println(steps(1), steps(6), steps(7), steps(27), steps(97))
	fmt.Println(peak(27), peak(77031))
	best, bestLen := uint64(0), 0
	for i := uint64(1); i < 100000; i++ {
		if s := steps(i); s > bestLen {
			best, bestLen = i, s
		}
	}
	fmt.Println(best, bestLen)
	n := uint64(12)
	for n != 1 {
		fmt.Print(n, " -> ")
		if n%2 == 0 {
			n /= 2
		} else {
			n = 3*n + 1
		}
	}
	fmt.Println(n)
}

// Output:
// 0 8 16 111 118
// 9232 21933016
// 77031 350
// 12 -> 6 -> 3 -> 10 -> 5 -> 16 -> 8 -> 4 -> 2 -> 1
